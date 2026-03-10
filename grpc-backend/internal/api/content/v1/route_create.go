package apicontentv1

import (
	"context"

	uuid "github.com/gofrs/uuid/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	contentv1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/content/v1"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcutil"
)

func (h *handler) CreateContent(
	ctx context.Context,
	req *contentv1.CreateContentRequest,
) (*contentv1.CreateContentResponse, error) {
	spaceID, err := uuid.FromString(req.GetSpace().GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid space.id: %v", err)
	}
	params := fromProtoCreate(req)
	params.SpaceID = spaceID
	result, err := h.service.Create(ctx, params)
	if err != nil {
		return nil, grpcutil.NewErrorFrom(err, errorMappings)
	}
	return &contentv1.CreateContentResponse{
		Content: toProto(result),
	}, nil
}
