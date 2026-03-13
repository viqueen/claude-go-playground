package apicontentv1

import (
	"context"

	uuid "github.com/gofrs/uuid/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	contentv1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/content/v1"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/grpcutil"
)

func (h *handler) UpdateContent(
	ctx context.Context,
	req *contentv1.UpdateContentRequest,
) (*contentv1.UpdateContentResponse, error) {
	id, err := uuid.FromString(req.GetId())
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid id: %v", err)
	}
	if contentID := req.GetContent().GetId(); contentID != "" && contentID != req.GetId() {
		return nil, status.Errorf(codes.InvalidArgument, "content.id %q does not match request id %q", contentID, req.GetId())
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
	return &contentv1.UpdateContentResponse{
		Content: toProto(result),
	}, nil
}
