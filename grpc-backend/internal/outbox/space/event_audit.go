package space

import (
	"context"

	"github.com/riverqueue/river"
	"github.com/rs/zerolog/log"

	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
)

const auditKind = "space.audit"

// AuditArgs represents a river job for auditing a space event.
type AuditArgs struct {
	EventType string `json:"event_type"`
	SpaceID   string `json:"space_id"`
}

func (AuditArgs) Kind() string { return auditKind }

// NewAuditArgs creates AuditArgs from an outbox event.
func NewAuditArgs(event outbox.Event) AuditArgs {
	return AuditArgs{
		EventType: event.Type,
		SpaceID:   event.ID,
	}
}

// AuditWorker processes space audit jobs.
type AuditWorker struct {
	river.WorkerDefaults[AuditArgs]
}

func (w *AuditWorker) Work(_ context.Context, job *river.Job[AuditArgs]) error {
	log.Info().
		Str("event_type", job.Args.EventType).
		Str("space_id", job.Args.SpaceID).
		Msg("auditing space event")
	return nil
}
