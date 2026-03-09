package space

import (
	"context"

	"github.com/riverqueue/river"
	"github.com/rs/zerolog/log"

	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
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

// IndexWorker processes space indexing jobs.
type IndexWorker struct {
	river.WorkerDefaults[IndexArgs]
}

func (w *IndexWorker) Work(_ context.Context, job *river.Job[IndexArgs]) error {
	log.Info().
		Str("event_type", job.Args.EventType).
		Str("space_id", job.Args.SpaceID).
		Msg("indexing space")
	return nil
}
