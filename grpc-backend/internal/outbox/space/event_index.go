package space

import (
	"context"
	"fmt"

	"github.com/gofrs/uuid/v5"
	"github.com/riverqueue/river"
	"github.com/rs/zerolog/log"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	spacedomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/space"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/embed"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/search"
)

const indexKind = "space.index"

// IndexArgs represents a river job for indexing a space.
type IndexArgs struct {
	EventType string `json:"event_type"`
	SpaceID   string `json:"space_id"`
}

func (IndexArgs) Kind() string { return indexKind }

// NewIndexArgs creates IndexArgs from an outbox event.
func NewIndexArgs(event outbox.Event) IndexArgs {
	return IndexArgs{
		EventType: event.Type,
		SpaceID:   event.ID,
	}
}

// IndexStore is the subset of db.Queries needed by the index worker.
type IndexStore interface {
	GetSpace(ctx context.Context, id uuid.UUID) (db.CollaborationSpace, error)
}

// IndexDependencies holds the dependencies for the space index worker.
type IndexDependencies struct {
	Search   search.Search
	Embedder embed.Embedder
	Queries  IndexStore
}

// IndexWorker defines the interface for the space index worker.
type IndexWorker interface {
	river.Worker[IndexArgs]
}

// NewIndexWorker creates a new IndexWorker with the given dependencies.
func NewIndexWorker(deps IndexDependencies) IndexWorker {
	return &indexWorker{
		search:   deps.Search,
		embedder: deps.Embedder,
		queries:  deps.Queries,
	}
}

type indexWorker struct {
	river.WorkerDefaults[IndexArgs]
	search   search.Search
	embedder embed.Embedder
	queries  IndexStore
}

func (w *indexWorker) Work(ctx context.Context, job *river.Job[IndexArgs]) error {
	id, err := uuid.FromString(job.Args.SpaceID)
	if err != nil {
		return err
	}

	switch job.Args.EventType {
	case spacedomain.EventCreated, spacedomain.EventUpdated:
		entity, err := w.queries.GetSpace(ctx, id)
		if err != nil {
			return err
		}

		doc := toDocument(&entity)

		embedding, err := w.embedder.Embed(ctx, doc.EmbeddingText())
		if err != nil {
			return err
		}
		if len(embedding) != EmbeddingDimension {
			return fmt.Errorf("search: embedding dimension mismatch: got %d, want %d", len(embedding), EmbeddingDimension)
		}
		doc.Embedding = embedding

		return w.search.Index(ctx, IndexName, id, doc)
	case spacedomain.EventDeleted:
		return w.search.Delete(ctx, IndexName, id)
	default:
		log.Ctx(ctx).Warn().Str("event_type", job.Args.EventType).Msg("unknown event type")
		return nil
	}
}
