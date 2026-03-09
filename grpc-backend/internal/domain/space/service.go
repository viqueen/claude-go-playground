package space

import (
	"context"

	uuid "github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/cache"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
)

// Service is the public interface for the space domain.
type Service interface {
	Create(ctx context.Context, params db.CreateSpaceParams) (*db.CollaborationSpace, error)
	Get(ctx context.Context, id uuid.UUID) (*db.CollaborationSpace, error)
	List(ctx context.Context, pageSize int32, pageToken string) ([]db.CollaborationSpace, string, error)
	Update(ctx context.Context, params db.UpdateSpaceParams) (*db.CollaborationSpace, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Dependencies holds the external dependencies for the space service.
type Dependencies struct {
	Pool    *pgxpool.Pool
	Queries *db.Queries
	Cache   cache.Cache[uuid.UUID, *db.CollaborationSpace]
	Outbox  outbox.Outbox[pgx.Tx]
}

// New creates a new space Service.
func New(deps Dependencies) Service {
	return &service{
		pool:    deps.Pool,
		queries: deps.Queries,
		cache:   deps.Cache,
		outbox:  deps.Outbox,
	}
}

type service struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	cache   cache.Cache[uuid.UUID, *db.CollaborationSpace]
	outbox  outbox.Outbox[pgx.Tx]
}
