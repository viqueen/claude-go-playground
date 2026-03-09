package testkit

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/migrate"
	"github.com/viqueen/claude-go-playground/grpc-backend/sql/migrations"
)

// SetupPostgres starts a postgres container and returns the connection string.
// The container is automatically terminated when the test completes.
func SetupPostgres(ctx context.Context, t *testing.T) string {
	t.Helper()

	pgContainer, err := postgres.Run(ctx,
		"postgres:17",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgContainer.Terminate(ctx) })

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	db, err := sql.Open("pgx", connStr)
	require.NoError(t, err)
	defer db.Close()

	err = migrate.Run(db, migrations.FS, ".")
	require.NoError(t, err)

	return connStr
}
