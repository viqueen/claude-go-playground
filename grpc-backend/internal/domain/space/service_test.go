package space_test

import (
	"context"
	"testing"

	uuid "github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	spacedomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/space"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/cache"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/testkit"
)

type noopOutbox[T any] struct{}

func (n *noopOutbox[T]) Emit(_ context.Context, _ T, _ ...outbox.Event) error {
	return nil
}

func setupService(t *testing.T) (spacedomain.Service, context.Context) {
	t.Helper()
	ctx := context.Background()
	connStr := testkit.SetupPostgres(ctx, t)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Close() })

	svc := spacedomain.New(spacedomain.Dependencies{
		Pool:    pool,
		Queries: db.New(pool),
		Cache:   cache.NewInMemory[uuid.UUID, *db.CollaborationSpace](),
		Outbox:  &noopOutbox[pgx.Tx]{},
	})
	return svc, ctx
}

func validCreateParams() db.CreateSpaceParams {
	return db.CreateSpaceParams{
		Name:        "Test Space",
		Key:         "TEST",
		Description: "A test space",
		Status:      1,
		Visibility:  1,
	}
}
