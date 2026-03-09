package collaboration

import (
	"context"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
)

func (s *service) ListContent(ctx context.Context, pageSize int32, pageToken string) ([]db.CollaborationContent, string, error) {
	offset, err := decodePageToken(pageToken)
	if err != nil {
		return nil, "", err
	}

	content, err := s.queries.ListContent(ctx, db.ListContentParams{
		Offset: offset,
		Limit:  pageSize,
	})
	if err != nil {
		return nil, "", err
	}

	return content, nextPageToken(offset, pageSize, len(content)), nil
}
