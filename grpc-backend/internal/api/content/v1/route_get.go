package apicontentv1

import (
	"context"

	uuid "github.com/gofrs/uuid/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	contentv1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/content/v1"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcutil"
)

func (h *handler) GetContent(
	ctx context.Context,
	req *contentv1.GetContentRequest,
) (*contentv1.GetContentResponse, error) {
	id, err := uuid.FromString(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}
	result, err := h.service.Get(ctx, id)
	if err != nil {
		return nil, grpcutil.NewErrorFrom(err, errorMappings)
	}
	return &contentv1.GetContentResponse{
		Content: toProto(result),
	}, nil
}
