package apispacev1

import (
	"context"

	uuid "github.com/gofrs/uuid/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcutil"
)

func (h *handler) DeleteSpace(
	ctx context.Context,
	req *spacev1.DeleteSpaceRequest,
) (*emptypb.Empty, error) {
	id, err := uuid.FromString(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}
	if err := h.service.Delete(ctx, id); err != nil {
		return nil, grpcutil.NewErrorFrom(err, errorMappings)
	}
	return &emptypb.Empty{}, nil
}
