package content

import (
	"context"

	uuid "github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/cache"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
)

// Service is the public interface for the content domain.
type Service interface {
	Create(ctx context.Context, params db.CreateContentParams) (*db.CollaborationContent, error)
	Get(ctx context.Context, id uuid.UUID) (*db.CollaborationContent, error)
	List(ctx context.Context, pageSize int32, pageToken string) ([]db.CollaborationContent, string, error)
	ListBySpace(ctx context.Context, spaceID uuid.UUID, pageSize int32, pageToken string) ([]db.CollaborationContent, string, error)
	Update(ctx context.Context, params db.UpdateContentParams) (*db.CollaborationContent, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// Dependencies holds the external dependencies for the content service.
type Dependencies struct {
	Pool    *pgxpool.Pool
	Queries *db.Queries
	Cache   cache.Cache[uuid.UUID, *db.CollaborationContent]
	Outbox  outbox.Outbox[pgx.Tx]
}

// New creates a new content Service.
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
	cache   cache.Cache[uuid.UUID, *db.CollaborationContent]
	outbox  outbox.Outbox[pgx.Tx]
}
