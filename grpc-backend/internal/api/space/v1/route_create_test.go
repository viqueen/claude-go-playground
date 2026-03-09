package apispacev1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
)

func TestCreateSpace_Errors(t *testing.T) {
	clients, ctx := setupHandler(t)

	t.Run("invalid argument — empty name", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
			Name:        "",
			Key:         "VALIDKEY",
			Description: "desc",
			Visibility:  spacev1.SpaceVisibility_SPACE_VISIBILITY_PRIVATE,
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid argument — invalid key format", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
			Name:        "Valid Name",
			Key:         "invalid-lowercase",
			Description: "desc",
			Visibility:  spacev1.SpaceVisibility_SPACE_VISIBILITY_PRIVATE,
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid argument — visibility unspecified", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
			Name:        "Valid Name",
			Key:         "VALIDKEY",
			Description: "desc",
			Visibility:  spacev1.SpaceVisibility_SPACE_VISIBILITY_UNSPECIFIED,
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestCreateSpace_Success(t *testing.T) {
	clients, ctx := setupHandlerWithDB(t)

	t.Run("already exists — duplicate key", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
			Name:        "First Space",
			Key:         "DUPKEY",
			Description: "first",
			Visibility:  spacev1.SpaceVisibility_SPACE_VISIBILITY_PRIVATE,
		})
		require.NoError(t, err)

		_, err = clients.standard.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
			Name:        "Second Space",
			Key:         "DUPKEY",
			Description: "second",
			Visibility:  spacev1.SpaceVisibility_SPACE_VISIBILITY_PRIVATE,
		})
		require.Error(t, err)
		assert.Equal(t, codes.AlreadyExists, status.Code(err))
	})

	t.Run("creates with required fields", func(t *testing.T) {
		t.Parallel()
		resp, err := clients.standard.CreateSpace(ctx, validCreateRequest())
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Space.Id)
		assert.Equal(t, "My Space", resp.Space.Name)
		assert.Equal(t, "MYSPACE", resp.Space.Key)
		assert.Equal(t, spacev1.SpaceStatus_SPACE_STATUS_DRAFT, resp.Space.Status)
		assert.Equal(t, spacev1.SpaceVisibility_SPACE_VISIBILITY_PRIVATE, resp.Space.Visibility)
	})

	t.Run("creates with public visibility", func(t *testing.T) {
		t.Parallel()
		resp, err := clients.standard.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
			Name:        "Public Space",
			Key:         "PUBLIC",
			Description: "open to all",
			Visibility:  spacev1.SpaceVisibility_SPACE_VISIBILITY_PUBLIC,
		})
		require.NoError(t, err)
		assert.Equal(t, spacev1.SpaceVisibility_SPACE_VISIBILITY_PUBLIC, resp.Space.Visibility)
		assert.Equal(t, "open to all", resp.Space.Description)
	})
}
