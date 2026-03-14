package apicontentv1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	contentv1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/content/v1"
)

func TestGetContent_Errors(t *testing.T) {
	clients, ctx := setupHandler(t)

	t.Run("invalid argument — bad UUID", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.GetContent(ctx, &contentv1.GetContentRequest{
			Id: "not-a-uuid",
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestGetContent_Success(t *testing.T) {
	tc := setupHandlerWithDB(t)

	t.Run("not found — nonexistent ID", func(t *testing.T) {
		t.Parallel()
		_, err := tc.clients.standard.GetContent(tc.ctx, &contentv1.GetContentRequest{
			Id: "00000000-0000-0000-0000-000000000001",
		})
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("returns created content", func(t *testing.T) {
		t.Parallel()
		spaceID := tc.createSpace(t)
		created, err := tc.clients.standard.CreateContent(tc.ctx, validCreateRequest(spaceID))
		require.NoError(t, err)

		resp, err := tc.clients.standard.GetContent(tc.ctx, &contentv1.GetContentRequest{
			Id: created.Content.Id,
		})
		require.NoError(t, err)
		assert.Equal(t, created.Content.Id, resp.Content.Id)
		assert.Equal(t, "My Content", resp.Content.Title)
		assert.Equal(t, "Some body text", resp.Content.Body)
		assert.Equal(t, spaceID, resp.Content.Space.Id)
	})
}
