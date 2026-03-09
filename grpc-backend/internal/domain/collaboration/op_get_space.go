package collaboration

import (
	"context"
	"errors"
	"time"

	uuid "github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
)

func (s *service) GetSpace(ctx context.Context, id uuid.UUID) (*db.CollaborationSpace, error) {
	if cached, ok := s.spaceCache.Get(id); ok {
		return cached, nil
	}

	space, err := s.queries.GetSpace(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSpaceNotFound
		}
		return nil, err
	}

	s.spaceCache.Set(id, &space, 5*time.Minute)
	return &space, nil
}
