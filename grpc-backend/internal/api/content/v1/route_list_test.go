package apicontentv1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	contentv1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/content/v1"
	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
)

func TestListContent_Errors(t *testing.T) {
	clients, ctx := setupHandler(t)

	t.Run("invalid argument — page size exceeds maximum", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.ListContent(ctx, &contentv1.ListContentRequest{
			PageSize: 101,
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid argument — negative page size", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.ListContent(ctx, &contentv1.ListContentRequest{
			PageSize: -1,
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestListContent_Success(t *testing.T) {
	tc := setupHandlerWithDB(t)

	t.Run("lists created content", func(t *testing.T) {
		t.Parallel()
		spaceID := tc.createSpace(t)
		_, err := tc.clients.standard.CreateContent(tc.ctx, &contentv1.CreateContentRequest{
			Space:  &spacev1.SpaceRef{Id: spaceID},
			Title:  "List Content A",
			Body:   "body a",
			Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
			Tags:   []string{},
		})
		require.NoError(t, err)

		_, err = tc.clients.standard.CreateContent(tc.ctx, &contentv1.CreateContentRequest{
			Space:  &spacev1.SpaceRef{Id: spaceID},
			Title:  "List Content B",
			Body:   "body b",
			Status: contentv1.ContentStatus_CONTENT_STATUS_PUBLISHED,
			Tags:   []string{},
		})
		require.NoError(t, err)

		resp, err := tc.clients.standard.ListContent(tc.ctx, &contentv1.ListContentRequest{
			PageSize: 10,
		})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp.Items), 2)
	})

	t.Run("paginates results", func(t *testing.T) {
		t.Parallel()
		spaceID := tc.createSpace(t)
		for i := 0; i < 3; i++ {
			_, err := tc.clients.standard.CreateContent(tc.ctx, &contentv1.CreateContentRequest{
				Space:  &spacev1.SpaceRef{Id: spaceID},
				Title:  "Page Content",
				Body:   "paginated body",
				Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
				Tags:   []string{},
			})
			require.NoError(t, err)
		}

		first, err := tc.clients.standard.ListContent(tc.ctx, &contentv1.ListContentRequest{
			PageSize: 1,
		})
		require.NoError(t, err)
		assert.Len(t, first.Items, 1)
		assert.NotEmpty(t, first.NextPageToken)

		second, err := tc.clients.standard.ListContent(tc.ctx, &contentv1.ListContentRequest{
			PageSize:  1,
			PageToken: first.NextPageToken,
		})
		require.NoError(t, err)
		assert.Len(t, second.Items, 1)
		assert.NotEqual(t, first.Items[0].Id, second.Items[0].Id)
	})
}
