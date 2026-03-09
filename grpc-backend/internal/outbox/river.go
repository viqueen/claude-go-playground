package outbox

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	spacedomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/space"
	spaceevents "github.com/viqueen/claude-go-playground/grpc-backend/internal/outbox/space"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
)

// NewRiverOutbox creates an Outbox backed by River.
func NewRiverOutbox(client *river.Client[pgx.Tx]) outbox.Outbox[pgx.Tx] {
	return &riverOutbox{client: client}
}

type riverOutbox struct {
	client *river.Client[pgx.Tx]
}

func (o *riverOutbox) Emit(ctx context.Context, tx pgx.Tx, events ...outbox.Event) error {
	for _, event := range events {
		jobs, err := o.mapEvent(event)
		if err != nil {
			return err
		}
		for _, args := range jobs {
			if _, err := o.client.InsertTx(ctx, tx, args, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// mapEvent fans out a domain event into one or more river jobs.
func (o *riverOutbox) mapEvent(event outbox.Event) ([]river.JobArgs, error) {
	switch event.Type {
	case spacedomain.EventCreated:
		return []river.JobArgs{
			spaceevents.NewIndexArgs(event),
			spaceevents.NewAuditArgs(event),
		}, nil
	case spacedomain.EventUpdated:
		return []river.JobArgs{
			spaceevents.NewIndexArgs(event),
			spaceevents.NewAuditArgs(event),
		}, nil
	case spacedomain.EventDeleted:
		return []river.JobArgs{
			spaceevents.NewIndexArgs(event),
			spaceevents.NewAuditArgs(event),
		}, nil
	case spacedomain.EventContentDeleted:
		return []river.JobArgs{
			spaceevents.NewIndexArgs(event),
			spaceevents.NewAuditArgs(event),
		}, nil
	default:
		return nil, fmt.Errorf("unknown event type: %s", event.Type)
	}
}
