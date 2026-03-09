package apispacev1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
)

func TestGetSpace_Errors(t *testing.T) {
	clients, ctx := setupHandler(t)

	t.Run("invalid argument — bad UUID", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.GetSpace(ctx, &spacev1.GetSpaceRequest{
			Id: "not-a-uuid",
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestGetSpace_Success(t *testing.T) {
	clients, ctx := setupHandlerWithDB(t)

	t.Run("not found — nonexistent ID", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.GetSpace(ctx, &spacev1.GetSpaceRequest{
			Id: "00000000-0000-0000-0000-000000000001",
		})
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("returns created space", func(t *testing.T) {
		t.Parallel()
		created, err := clients.standard.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
			Name:        "Get Test Space",
			Key:         "GETTEST",
			Description: "for get test",
			Visibility:  spacev1.SpaceVisibility_SPACE_VISIBILITY_PRIVATE,
		})
		require.NoError(t, err)

		resp, err := clients.standard.GetSpace(ctx, &spacev1.GetSpaceRequest{
			Id: created.Space.Id,
		})
		require.NoError(t, err)
		assert.Equal(t, created.Space.Id, resp.Space.Id)
		assert.Equal(t, "Get Test Space", resp.Space.Name)
		assert.Equal(t, "GETTEST", resp.Space.Key)
	})
}
