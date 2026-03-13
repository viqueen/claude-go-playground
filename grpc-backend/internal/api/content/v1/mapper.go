package apicontentv1

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	contentv1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/content/v1"
	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
)

var supportedUpdatePaths = map[string]bool{
	"title":  true,
	"body":   true,
	"status": true,
	"tags":   true,
}

func toProto(model *db.CollaborationContent) *contentv1.Content {
	return &contentv1.Content{
		Id:        model.ID.String(),
		Space:     &spacev1.SpaceRef{Id: model.SpaceID.String()},
		Title:     model.Title,
		Body:      model.Body,
		Status:    contentv1.ContentStatus(model.Status),
		Tags:      model.Tags,
		CreatedAt: timestamppb.New(model.CreatedAt),
		UpdatedAt: timestamppb.New(model.UpdatedAt),
	}
}

func toProtoList(models []db.CollaborationContent) []*contentv1.Content {
	items := make([]*contentv1.Content, len(models))
	for i := range models {
		items[i] = toProto(&models[i])
	}
	return items
}

func fromProtoCreate(req *contentv1.CreateContentRequest) db.CreateContentParams {
	return db.CreateContentParams{
		Title:  req.GetTitle(),
		Body:   req.GetBody(),
		Status: int32(req.GetStatus()),
		Tags:   req.GetTags(),
	}
}

func validateUpdateMask(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("update_mask must contain at least one path")
	}
	for _, path := range paths {
		if !supportedUpdatePaths[path] {
			return fmt.Errorf("unsupported update_mask path: %q", path)
		}
	}
	return nil
}

func fromProtoUpdate(req *contentv1.UpdateContentRequest) db.UpdateContentParams {
	params := db.UpdateContentParams{}

	for _, path := range req.GetUpdateMask().GetPaths() {
		switch path {
		case "title":
			params.Title = pgtype.Text{String: req.GetContent().GetTitle(), Valid: true}
		case "body":
			params.Body = pgtype.Text{String: req.GetContent().GetBody(), Valid: true}
		case "status":
			params.Status = pgtype.Int4{Int32: int32(req.GetContent().GetStatus()), Valid: true}
		case "tags":
			params.Tags = req.GetContent().GetTags()
		}
	}

	return params
}
