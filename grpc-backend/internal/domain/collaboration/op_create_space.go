package collaboration

import (
	"context"
	"time"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
)

func (s *service) CreateSpace(ctx context.Context, params db.CreateSpaceParams) (*db.CollaborationSpace, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	space, err := s.queries.WithTx(tx).CreateSpace(ctx, params)
	if err != nil {
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

	s.spaceCache.Set(space.ID, &space, 5*time.Minute)
	return &space, nil
}
