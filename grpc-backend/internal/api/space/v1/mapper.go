package apispacev1

import (
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
)

func toProto(model *db.CollaborationSpace) *spacev1.Space {
	return &spacev1.Space{
		Id:          model.ID.String(),
		Name:        model.Name,
		Key:         model.Key,
		Description: model.Description,
		Status:      spacev1.SpaceStatus(model.Status),
		Visibility:  spacev1.SpaceVisibility(model.Visibility),
		CreatedAt:   timestamppb.New(model.CreatedAt),
		UpdatedAt:   timestamppb.New(model.UpdatedAt),
	}
}

func toProtoList(models []db.CollaborationSpace) []*spacev1.Space {
	items := make([]*spacev1.Space, len(models))
	for i := range models {
		items[i] = toProto(&models[i])
	}
	return items
}

func fromProtoCreate(req *spacev1.CreateSpaceRequest) db.CreateSpaceParams {
	return db.CreateSpaceParams{
		Name:        req.GetName(),
		Key:         req.GetKey(),
		Description: req.GetDescription(),
		Status:      int32(spacev1.SpaceStatus_SPACE_STATUS_DRAFT),
		Visibility:  int32(req.GetVisibility()),
	}
}

func fromProtoUpdate(req *spacev1.UpdateSpaceRequest) db.UpdateSpaceParams {
	params := db.UpdateSpaceParams{}

	for _, path := range req.GetUpdateMask().GetPaths() {
		switch path {
		case "name":
			params.Name = pgtype.Text{String: req.GetSpace().GetName(), Valid: true}
		case "key":
			params.Key = pgtype.Text{String: req.GetSpace().GetKey(), Valid: true}
		case "description":
			params.Description = pgtype.Text{String: req.GetSpace().GetDescription(), Valid: true}
		case "status":
			params.Status = pgtype.Int4{Int32: int32(req.GetSpace().GetStatus()), Valid: true}
		case "visibility":
			params.Visibility = pgtype.Int4{Int32: int32(req.GetSpace().GetVisibility()), Valid: true}
		}
	}

	return params
}
