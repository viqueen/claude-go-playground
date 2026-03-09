package apispacev1

import (
	"context"

	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcutil"
)

func (h *handler) CreateSpace(
	ctx context.Context,
	req *spacev1.CreateSpaceRequest,
) (*spacev1.CreateSpaceResponse, error) {
	result, err := h.service.Create(ctx, fromProtoCreate(req))
	if err != nil {
		return nil, grpcutil.NewErrorFrom(err, errorMappings)
	}
	return &spacev1.CreateSpaceResponse{
		Space: toProto(result),
	}, nil
}
