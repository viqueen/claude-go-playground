package space

import (
	"context"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/pagination"
)

func (s *service) List(ctx context.Context, pageSize int32, pageToken string) ([]db.CollaborationSpace, string, error) {
	offset, err := pagination.DecodePageToken(pageToken)
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

	return spaces, pagination.NextPageToken(offset, pageSize, len(spaces)), nil
}
