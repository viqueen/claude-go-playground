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

	spaceevents "github.com/viqueen/claude-go-playground/grpc-backend/internal/outbox/space"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/config"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/migrate"
	migrations "github.com/viqueen/claude-go-playground/grpc-backend/sql/migrations"
)

type Connections struct {
	Pool        *pgxpool.Pool
	RiverClient *river.Client[pgx.Tx]
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

	// River client with domain workers
	workers := river.NewWorkers()
	river.AddWorker(workers, &spaceevents.IndexWorker{})
	river.AddWorker(workers, &spaceevents.AuditWorker{})

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

	return &Connections{Pool: pool, RiverClient: riverClient}
}
