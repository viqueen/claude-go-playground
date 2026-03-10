package content

import (
	"context"

	"github.com/riverqueue/river"
	"github.com/rs/zerolog/log"

	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
)

const auditKind = "content.audit"

// AuditArgs represents a river job for auditing a content event.
type AuditArgs struct {
	EventType string `json:"event_type"`
	ContentID string `json:"content_id"`
}

func (AuditArgs) Kind() string { return auditKind }

// NewAuditArgs creates AuditArgs from an outbox event.
func NewAuditArgs(event outbox.Event) AuditArgs {
	return AuditArgs{
		EventType: event.Type,
		ContentID: event.ID,
	}
}

// AuditWorker processes content audit jobs.
type AuditWorker struct {
	river.WorkerDefaults[AuditArgs]
}

func (w *AuditWorker) Work(ctx context.Context, job *river.Job[AuditArgs]) error {
	log.Ctx(ctx).Info().
		Str("event_type", job.Args.EventType).
		Str("content_id", job.Args.ContentID).
		Msg("auditing content event")
	return nil
}
