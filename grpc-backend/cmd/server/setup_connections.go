package main

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/rs/zerolog/log"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/config"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/migrate"
	migrations "github.com/viqueen/claude-go-playground/grpc-backend/sql/migrations"
)

type Connections struct {
	Pool        *pgxpool.Pool
	RiverClient *river.Client[pgx.Tx]
}

func (c *Connections) Close(ctx context.Context) {
	if err := c.RiverClient.Stop(ctx); err != nil {
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

	// River client — no workers registered yet, domains add them via the integrate agent
	workers := river.NewWorkers()
	river.AddWorker(workers, &placeholderWorker{})

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

// placeholderWorker satisfies River's requirement of at least one worker.
// Domains add real workers via the integrate agent.
type placeholderArgs struct{}

func (placeholderArgs) Kind() string { return "placeholder" }

type placeholderWorker struct {
	river.WorkerDefaults[placeholderArgs]
}

func (w *placeholderWorker) Work(_ context.Context, _ *river.Job[placeholderArgs]) error {
	return nil
}
