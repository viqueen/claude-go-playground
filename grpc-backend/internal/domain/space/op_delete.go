package space

import (
	"context"
	"errors"

	uuid "github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"

	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
)

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	txQueries := s.queries.WithTx(tx)

	space, err := txQueries.SoftDeleteSpace(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if err := txQueries.SoftDeleteContentBySpace(ctx, id); err != nil {
		return err
	}

	if err := s.outbox.Emit(ctx, tx,
		outbox.Event{
			Type: EventContentDeleted,
			ID:   id.String(),
			Data: map[string]string{"space_id": id.String()},
		},
		outbox.Event{
			Type: EventDeleted,
			ID:   space.ID.String(),
			Data: space,
		},
	); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	s.cache.Delete(id)
	return nil
}
