package collaboration

import (
	"context"
	"errors"

	uuid "github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"

	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
)

func (s *service) DeleteContent(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	content, err := s.queries.WithTx(tx).SoftDeleteContent(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrContentNotFound
		}
		return err
	}

	if err := s.outbox.Emit(ctx, tx, outbox.Event{
		Type: "content.deleted",
		ID:   content.ID.String(),
		Data: content,
	}); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	s.contentCache.Delete(id)
	return nil
}
