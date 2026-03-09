package apispacev1

import (
	"context"

	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcutil"
)

func (h *handler) ListSpaces(
	ctx context.Context,
	req *spacev1.ListSpacesRequest,
) (*spacev1.ListSpacesResponse, error) {
	spaces, nextToken, err := h.service.List(ctx, req.GetPageSize(), req.GetPageToken())
	if err != nil {
		return nil, grpcutil.NewErrorFrom(err, errorMappings)
	}
	return &spacev1.ListSpacesResponse{
		Items:         toProtoList(spaces),
		NextPageToken: nextToken,
	}, nil
}
