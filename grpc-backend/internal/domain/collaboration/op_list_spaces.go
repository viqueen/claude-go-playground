package collaboration

import (
	"context"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
)

func (s *service) ListSpaces(ctx context.Context, pageSize int32, pageToken string) ([]db.CollaborationSpace, string, error) {
	offset, err := decodePageToken(pageToken)
	if err != nil {
		return nil, "", err
	}

	spaces, err := s.queries.ListSpaces(ctx, db.ListSpacesParams{
		Offset: offset,
		Limit:  pageSize,
	})
	if err != nil {
		return nil, "", err
	}

	return spaces, nextPageToken(offset, pageSize, len(spaces)), nil
}
