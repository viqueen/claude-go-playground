package apicontentv1

import (
	"context"

	uuid "github.com/gofrs/uuid/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	contentv1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/content/v1"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcutil"
)

func (h *handler) DeleteContent(
	ctx context.Context,
	req *contentv1.DeleteContentRequest,
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
