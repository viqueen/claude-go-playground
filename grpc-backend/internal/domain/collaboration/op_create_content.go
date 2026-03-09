package collaboration

import (
	"context"
	"time"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
)

func (s *service) CreateContent(ctx context.Context, params db.CreateContentParams) (*db.CollaborationContent, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	content, err := s.queries.WithTx(tx).CreateContent(ctx, params)
	if err != nil {
		return nil, err
	}

	if err := s.outbox.Emit(ctx, tx, outbox.Event{
		Type: "content.created",
		ID:   content.ID.String(),
		Data: content,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	s.contentCache.Set(content.ID, &content, 5*time.Minute)
	return &content, nil
}
