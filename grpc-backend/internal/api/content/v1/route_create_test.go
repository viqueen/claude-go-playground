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

func TestCreateContent_Errors(t *testing.T) {
	clients, ctx := setupHandler(t)

	t.Run("invalid argument — missing space", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.CreateContent(ctx, &contentv1.CreateContentRequest{
			Title:  "Valid Title",
			Body:   "Valid body",
			Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid argument — empty title", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.CreateContent(ctx, &contentv1.CreateContentRequest{
			Space:  &spacev1.SpaceRef{Id: "00000000-0000-0000-0000-000000000001"},
			Title:  "",
			Body:   "Valid body",
			Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid argument — empty body", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.CreateContent(ctx, &contentv1.CreateContentRequest{
			Space:  &spacev1.SpaceRef{Id: "00000000-0000-0000-0000-000000000001"},
			Title:  "Valid Title",
			Body:   "",
			Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid argument — status unspecified", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.CreateContent(ctx, &contentv1.CreateContentRequest{
			Space:  &spacev1.SpaceRef{Id: "00000000-0000-0000-0000-000000000001"},
			Title:  "Valid Title",
			Body:   "Valid body",
			Status: contentv1.ContentStatus_CONTENT_STATUS_UNSPECIFIED,
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid argument — invalid space UUID", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.CreateContent(ctx, &contentv1.CreateContentRequest{
			Space:  &spacev1.SpaceRef{Id: "not-a-uuid"},
			Title:  "Valid Title",
			Body:   "Valid body",
			Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestCreateContent_Success(t *testing.T) {
	tc := setupHandlerWithDB(t)

	t.Run("creates with required fields", func(t *testing.T) {
		t.Parallel()
		spaceID := tc.createSpace(t)
		resp, err := tc.clients.standard.CreateContent(tc.ctx, validCreateRequest(spaceID))
		require.NoError(t, err)
		assert.NotEmpty(t, resp.Content.Id)
		assert.Equal(t, "My Content", resp.Content.Title)
		assert.Equal(t, "Some body text", resp.Content.Body)
		assert.Equal(t, contentv1.ContentStatus_CONTENT_STATUS_DRAFT, resp.Content.Status)
		assert.Equal(t, spaceID, resp.Content.Space.Id)
	})

	t.Run("creates with optional tags", func(t *testing.T) {
		t.Parallel()
		spaceID := tc.createSpace(t)
		resp, err := tc.clients.standard.CreateContent(tc.ctx, &contentv1.CreateContentRequest{
			Space:  &spacev1.SpaceRef{Id: spaceID},
			Title:  "Tagged Content",
			Body:   "body with tags",
			Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
			Tags:   []string{"go", "grpc"},
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"go", "grpc"}, resp.Content.Tags)
	})

	t.Run("creates without tags — defaults to empty", func(t *testing.T) {
		t.Parallel()
		spaceID := tc.createSpace(t)
		resp, err := tc.clients.standard.CreateContent(tc.ctx, &contentv1.CreateContentRequest{
			Space:  &spacev1.SpaceRef{Id: spaceID},
			Title:  "No Tags Content",
			Body:   "body without tags",
			Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
		})
		require.NoError(t, err)
		assert.Empty(t, resp.Content.Tags)
	})

	t.Run("creates with published status", func(t *testing.T) {
		t.Parallel()
		spaceID := tc.createSpace(t)
		resp, err := tc.clients.standard.CreateContent(tc.ctx, &contentv1.CreateContentRequest{
			Space:  &spacev1.SpaceRef{Id: spaceID},
			Title:  "Published Content",
			Body:   "published body",
			Status: contentv1.ContentStatus_CONTENT_STATUS_PUBLISHED,
			Tags:   []string{},
		})
		require.NoError(t, err)
		assert.Equal(t, contentv1.ContentStatus_CONTENT_STATUS_PUBLISHED, resp.Content.Status)
	})
}
