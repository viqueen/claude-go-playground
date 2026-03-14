package main

import (
	"context"
	"database/sql"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/rs/zerolog/log"

	_ "github.com/jackc/pgx/v5/stdlib"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	contentevents "github.com/viqueen/claude-go-playground/grpc-backend/internal/outbox/content"
	spaceevents "github.com/viqueen/claude-go-playground/grpc-backend/internal/outbox/space"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/config"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/embed"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/migrate"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/search"
	migrations "github.com/viqueen/claude-go-playground/grpc-backend/sql/migrations"
)

type Connections struct {
	Pool         *pgxpool.Pool
	RiverClient  *river.Client[pgx.Tx]
	SearchClient search.Search
	Embedder     embed.Embedder
}

func (c *Connections) Close() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.RiverClient.Stop(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("failed to stop river client")
	}
	c.Pool.Close()
}

func setupConnections(ctx context.Context, cfg *config.Config) *Connections {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}

	// River migrations
	riverMigrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create river migrator")
	}
	if _, err := riverMigrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		log.Fatal().Err(err).Msg("failed to run river migrations")
	}

	// Domain migrations (goose)
	stdDB, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to open sql connection")
	}
	if err := migrate.Run(stdDB, migrations.FS, "."); err != nil {
		log.Fatal().Err(err).Msg("failed to run domain migrations")
	}
	stdDB.Close()

	// Search client
	searchClient, err := search.New(cfg.OpenSearchURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create search client")
	}

	// Embedder
	if cfg.EmbedModelID == "" {
		log.Fatal().Msg("EMBED_MODEL_ID is required for embedding support")
	}
	embedder, err := embed.NewOpenSearch(cfg.OpenSearchURL, cfg.EmbedModelID)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create embedder")
	}

	// Create indexes on startup
	if err := searchClient.CreateIndexIfNotExists(ctx, spaceevents.IndexName, spaceevents.IndexMapping); err != nil {
		log.Fatal().Err(err).Msg("failed to create spaces index")
	}

	// River client with domain workers
	queries := db.New(pool)
	workers := river.NewWorkers()
	river.AddWorker(workers, spaceevents.NewIndexWorker(spaceevents.IndexDependencies{
		Search:   searchClient,
		Embedder: embedder,
		Queries:  queries,
	}))
	river.AddWorker(workers, &spaceevents.AuditWorker{})
	river.AddWorker(workers, &contentevents.IndexWorker{})
	river.AddWorker(workers, &contentevents.AuditWorker{})

	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 100}},
		Workers: workers,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create river client")
	}
	if err := riverClient.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start river client")
	}

	return &Connections{
		Pool:         pool,
		RiverClient:  riverClient,
		SearchClient: searchClient,
		Embedder:     embedder,
	}
}
