package space

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
)

func (s *service) Create(ctx context.Context, params db.CreateSpaceParams) (*db.CollaborationSpace, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	space, err := s.queries.WithTx(tx).CreateSpace(ctx, params)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return nil, ErrAlreadyExists
		}
		return nil, err
	}

	if err := s.outbox.Emit(ctx, tx, outbox.Event{
		Type: "space.created",
		ID:   space.ID.String(),
		Data: space,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	s.cache.Set(space.ID, &space, 5*time.Minute)
	return &space, nil
}
