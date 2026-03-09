package collaboration

import (
	"context"

	uuid "github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/cache"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
)

// Service is the public interface for the collaboration domain.
type Service interface {
	// Space operations
	CreateSpace(ctx context.Context, params db.CreateSpaceParams) (*db.CollaborationSpace, error)
	GetSpace(ctx context.Context, id uuid.UUID) (*db.CollaborationSpace, error)
	ListSpaces(ctx context.Context, pageSize int32, pageToken string) ([]db.CollaborationSpace, string, error)
	UpdateSpace(ctx context.Context, params db.UpdateSpaceParams) (*db.CollaborationSpace, error)
	DeleteSpace(ctx context.Context, id uuid.UUID) error

	// Content operations
	CreateContent(ctx context.Context, params db.CreateContentParams) (*db.CollaborationContent, error)
	GetContent(ctx context.Context, id uuid.UUID) (*db.CollaborationContent, error)
	ListContent(ctx context.Context, pageSize int32, pageToken string) ([]db.CollaborationContent, string, error)
	ListContentBySpace(ctx context.Context, spaceID uuid.UUID, pageSize int32, pageToken string) ([]db.CollaborationContent, string, error)
	UpdateContent(ctx context.Context, params db.UpdateContentParams) (*db.CollaborationContent, error)
	DeleteContent(ctx context.Context, id uuid.UUID) error
}

// Dependencies holds the external dependencies for the collaboration service.
type Dependencies struct {
	Pool         *pgxpool.Pool
	Queries      *db.Queries
	SpaceCache   cache.Cache[uuid.UUID, *db.CollaborationSpace]
	ContentCache cache.Cache[uuid.UUID, *db.CollaborationContent]
	Outbox       outbox.Outbox[pgx.Tx]
}

// New creates a new collaboration Service.
func New(deps Dependencies) Service {
	return &service{
		pool:         deps.Pool,
		queries:      deps.Queries,
		spaceCache:   deps.SpaceCache,
		contentCache: deps.ContentCache,
		outbox:       deps.Outbox,
	}
}

type service struct {
	pool         *pgxpool.Pool
	queries      *db.Queries
	spaceCache   cache.Cache[uuid.UUID, *db.CollaborationSpace]
	contentCache cache.Cache[uuid.UUID, *db.CollaborationContent]
	outbox       outbox.Outbox[pgx.Tx]
}
