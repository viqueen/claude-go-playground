package apispacev1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
)

func TestUpdateSpace_Errors(t *testing.T) {
	clients, ctx := setupHandler(t)

	t.Run("invalid argument — bad UUID", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{
			Id:         "not-a-uuid",
			Space:      &spacev1.Space{Name: "Updated", Key: "VALID"},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name"}},
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid argument — missing space", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{
			Id:         "00000000-0000-0000-0000-000000000001",
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name"}},
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid argument — missing update mask", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{
			Id:    "00000000-0000-0000-0000-000000000001",
			Space: &spacev1.Space{Name: "Updated", Key: "VALID"},
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid argument — unsupported update mask path", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{
			Id:         "00000000-0000-0000-0000-000000000001",
			Space:      &spacev1.Space{Name: "Updated", Key: "VALID"},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"invalid_path"}},
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestUpdateSpace_Success(t *testing.T) {
	clients, ctx := setupHandlerWithDB(t)

	t.Run("not found — nonexistent ID", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{
			Id:         "00000000-0000-0000-0000-000000000001",
			Space:      &spacev1.Space{Name: "Updated", Key: "NOTFOUND"},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name"}},
		})
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("updates name", func(t *testing.T) {
		t.Parallel()
		created, err := clients.standard.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
			Name:        "Before Update",
			Key:         "UPDTNAME",
			Description: "original",
			Visibility:  spacev1.SpaceVisibility_SPACE_VISIBILITY_PRIVATE,
		})
		require.NoError(t, err)

		resp, err := clients.standard.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{
			Id:         created.Space.Id,
			Space:      &spacev1.Space{Name: "After Update", Key: "UPDTNAME"},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name"}},
		})
		require.NoError(t, err)
		assert.Equal(t, "After Update", resp.Space.Name)
		assert.Equal(t, "UPDTNAME", resp.Space.Key)
	})

	t.Run("updates multiple fields", func(t *testing.T) {
		t.Parallel()
		created, err := clients.standard.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
			Name:        "Multi Update",
			Key:         "MULTI",
			Description: "original desc",
			Visibility:  spacev1.SpaceVisibility_SPACE_VISIBILITY_PRIVATE,
		})
		require.NoError(t, err)

		resp, err := clients.standard.UpdateSpace(ctx, &spacev1.UpdateSpaceRequest{
			Id: created.Space.Id,
			Space: &spacev1.Space{
				Name:        "Multi Update",
				Key:         "MULTI",
				Description: "new desc",
				Visibility:  spacev1.SpaceVisibility_SPACE_VISIBILITY_PUBLIC,
			},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"description", "visibility"}},
		})
		require.NoError(t, err)
		assert.Equal(t, "new desc", resp.Space.Description)
		assert.Equal(t, spacev1.SpaceVisibility_SPACE_VISIBILITY_PUBLIC, resp.Space.Visibility)
		assert.Equal(t, "Multi Update", resp.Space.Name)
	})
}
