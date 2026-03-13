package content

import (
	"context"

	"github.com/riverqueue/river"
	"github.com/rs/zerolog/log"

	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
)

const indexKind = "content.index"

// IndexArgs represents a river job for indexing content.
type IndexArgs struct {
	EventType string `json:"event_type"`
	ContentID string `json:"content_id"`
}

func (IndexArgs) Kind() string { return indexKind }

// NewIndexArgs creates IndexArgs from an outbox event.
func NewIndexArgs(event outbox.Event) IndexArgs {
	return IndexArgs{
		EventType: event.Type,
		ContentID: event.ID,
	}
}

// IndexWorker processes content indexing jobs.
type IndexWorker struct {
	river.WorkerDefaults[IndexArgs]
}

func (w *IndexWorker) Work(ctx context.Context, job *river.Job[IndexArgs]) error {
	log.Ctx(ctx).Info().
		Str("event_type", job.Args.EventType).
		Str("content_id", job.Args.ContentID).
		Msg("indexing content")
	return nil
}
