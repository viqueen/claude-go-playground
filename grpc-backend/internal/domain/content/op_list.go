package content

import (
	"context"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/pagination"
)

func (s *service) List(ctx context.Context, pageSize int32, pageToken string) ([]db.CollaborationContent, string, error) {
	offset, err := pagination.DecodePageToken(pageToken)
	if err != nil {
		return nil, "", err
	}

	items, err := s.queries.ListContent(ctx, db.ListContentParams{
		Offset: offset,
		Limit:  pageSize,
	})
	if err != nil {
		return nil, "", err
	}

	return items, pagination.NextPageToken(offset, pageSize, len(items)), nil
}
