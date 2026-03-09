package apispacev1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
)

func TestListSpaces_Errors(t *testing.T) {
	clients, ctx := setupHandler(t)

	t.Run("invalid argument — page size exceeds maximum", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.ListSpaces(ctx, &spacev1.ListSpacesRequest{
			PageSize: 101,
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid argument — negative page size", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.ListSpaces(ctx, &spacev1.ListSpacesRequest{
			PageSize: -1,
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestListSpaces_Success(t *testing.T) {
	clients, ctx := setupHandlerWithDB(t)

	t.Run("lists created spaces", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
			Name:        "List Space A",
			Key:         "LISTA",
			Description: "a",
			Visibility:  spacev1.SpaceVisibility_SPACE_VISIBILITY_PRIVATE,
		})
		require.NoError(t, err)

		_, err = clients.standard.CreateSpace(ctx, &spacev1.CreateSpaceRequest{
			Name:        "List Space B",
			Key:         "LISTB",
			Description: "b",
			Visibility:  spacev1.SpaceVisibility_SPACE_VISIBILITY_INTERNAL,
		})
		require.NoError(t, err)

		resp, err := clients.standard.ListSpaces(ctx, &spacev1.ListSpacesRequest{
			PageSize: 10,
		})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp.Items), 2)
	})
}
