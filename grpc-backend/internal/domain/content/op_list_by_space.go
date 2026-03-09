package content

import (
	"context"

	uuid "github.com/gofrs/uuid/v5"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
)

func (s *service) ListBySpace(ctx context.Context, spaceID uuid.UUID, pageSize int32, pageToken string) ([]db.CollaborationContent, string, error) {
	offset, err := decodePageToken(pageToken)
	if err != nil {
		return nil, "", err
	}

	items, err := s.queries.ListContentBySpace(ctx, db.ListContentBySpaceParams{
		SpaceID: spaceID,
		Offset:  offset,
		Limit:   pageSize,
	})
	if err != nil {
		return nil, "", err
	}

	return items, nextPageToken(offset, pageSize, len(items)), nil
}
