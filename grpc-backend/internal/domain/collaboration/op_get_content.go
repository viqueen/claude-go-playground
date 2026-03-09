package collaboration

import (
	"context"
	"errors"
	"time"

	uuid "github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
)

func (s *service) GetContent(ctx context.Context, id uuid.UUID) (*db.CollaborationContent, error) {
	if cached, ok := s.contentCache.Get(id); ok {
		return cached, nil
	}

	content, err := s.queries.GetContent(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrContentNotFound
		}
		return nil, err
	}

	s.contentCache.Set(id, &content, 5*time.Minute)
	return &content, nil
}
