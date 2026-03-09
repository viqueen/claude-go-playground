package collaboration

import (
	"context"

	uuid "github.com/gofrs/uuid/v5"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
)

func (s *service) ListContentBySpace(ctx context.Context, spaceID uuid.UUID, pageSize int32, pageToken string) ([]db.CollaborationContent, string, error) {
	offset, err := decodePageToken(pageToken)
	if err != nil {
		return nil, "", err
	}

	content, err := s.queries.ListContentBySpace(ctx, db.ListContentBySpaceParams{
		SpaceID: spaceID,
		Offset:  offset,
		Limit:   pageSize,
	})
	if err != nil {
		return nil, "", err
	}

	return content, nextPageToken(offset, pageSize, len(content)), nil
}
