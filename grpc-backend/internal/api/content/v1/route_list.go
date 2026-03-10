package apicontentv1

import (
	"context"

	contentv1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/content/v1"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcutil"
)

func (h *handler) ListContent(
	ctx context.Context,
	req *contentv1.ListContentRequest,
) (*contentv1.ListContentResponse, error) {
	items, nextToken, err := h.service.List(ctx, req.GetPageSize(), req.GetPageToken())
	if err != nil {
		return nil, grpcutil.NewErrorFrom(err, errorMappings)
	}
	return &contentv1.ListContentResponse{
		Items:         toProtoList(items),
		NextPageToken: nextToken,
	}, nil
}
