package apispacev1

import (
	"context"

	uuid "github.com/gofrs/uuid/v5"

	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcutil"
)

func (h *handler) GetSpace(
	ctx context.Context,
	req *spacev1.GetSpaceRequest,
) (*spacev1.GetSpaceResponse, error) {
	id, err := uuid.FromString(req.GetId())
	if err != nil {
		return nil, grpcutil.NewErrorFrom(err, errorMappings)
	}
	result, err := h.service.Get(ctx, id)
	if err != nil {
		return nil, grpcutil.NewErrorFrom(err, errorMappings)
	}
	return &spacev1.GetSpaceResponse{
		Space: toProto(result),
	}, nil
}
