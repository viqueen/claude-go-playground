package apispacev1

import (
	"context"

	uuid "github.com/gofrs/uuid/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcutil"
)

func (h *handler) UpdateSpace(
	ctx context.Context,
	req *spacev1.UpdateSpaceRequest,
) (*spacev1.UpdateSpaceResponse, error) {
	id, err := uuid.FromString(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}
	if spaceID := req.GetSpace().GetId(); spaceID != "" && spaceID != req.GetId() {
		return nil, status.Errorf(codes.InvalidArgument, "space.id %q does not match request id %q", spaceID, req.GetId())
	}
	if err := validateUpdateMask(req.GetUpdateMask().GetPaths()); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}
	params := fromProtoUpdate(req)
	params.ID = id
	result, err := h.service.Update(ctx, params)
	if err != nil {
		return nil, grpcutil.NewErrorFrom(err, errorMappings)
	}
	return &spacev1.UpdateSpaceResponse{
		Space: toProto(result),
	}, nil
}
