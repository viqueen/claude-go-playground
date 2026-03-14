package apicontentv1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	contentv1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/content/v1"
	spacev1 "github.com/viqueen/claude-go-playground/grpc-backend/gen/sdk/space/v1"
)

func TestUpdateContent_Errors(t *testing.T) {
	clients, ctx := setupHandler(t)

	t.Run("invalid argument — bad UUID", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.UpdateContent(ctx, &contentv1.UpdateContentRequest{
			Id: "not-a-uuid",
			Content: &contentv1.Content{
				Title:  "Updated",
				Body:   "updated body",
				Space:  &spacev1.SpaceRef{Id: "00000000-0000-0000-0000-000000000001"},
				Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
			},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"title"}},
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid argument — missing content", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.UpdateContent(ctx, &contentv1.UpdateContentRequest{
			Id:         "00000000-0000-0000-0000-000000000001",
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"title"}},
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid argument — missing update mask", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.UpdateContent(ctx, &contentv1.UpdateContentRequest{
			Id: "00000000-0000-0000-0000-000000000001",
			Content: &contentv1.Content{
				Title:  "Updated",
				Body:   "updated body",
				Space:  &spacev1.SpaceRef{Id: "00000000-0000-0000-0000-000000000001"},
				Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
			},
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("invalid argument — unsupported update mask path", func(t *testing.T) {
		t.Parallel()
		_, err := clients.standard.UpdateContent(ctx, &contentv1.UpdateContentRequest{
			Id: "00000000-0000-0000-0000-000000000001",
			Content: &contentv1.Content{
				Title:  "Updated",
				Body:   "updated body",
				Space:  &spacev1.SpaceRef{Id: "00000000-0000-0000-0000-000000000001"},
				Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
			},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"invalid_path"}},
		})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestUpdateContent_Success(t *testing.T) {
	tc := setupHandlerWithDB(t)

	t.Run("not found — nonexistent ID", func(t *testing.T) {
		t.Parallel()
		_, err := tc.clients.standard.UpdateContent(tc.ctx, &contentv1.UpdateContentRequest{
			Id: "00000000-0000-0000-0000-000000000001",
			Content: &contentv1.Content{
				Title:  "Updated",
				Body:   "updated body",
				Space:  &spacev1.SpaceRef{Id: "00000000-0000-0000-0000-000000000001"},
				Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
			},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"title"}},
		})
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("updates title", func(t *testing.T) {
		t.Parallel()
		spaceID := tc.createSpace(t)
		created, err := tc.clients.standard.CreateContent(tc.ctx, validCreateRequest(spaceID))
		require.NoError(t, err)

		resp, err := tc.clients.standard.UpdateContent(tc.ctx, &contentv1.UpdateContentRequest{
			Id: created.Content.Id,
			Content: &contentv1.Content{
				Title:  "Updated Title",
				Body:   "This body should be ignored by the mask",
				Space:  &spacev1.SpaceRef{Id: spaceID},
				Status: contentv1.ContentStatus_CONTENT_STATUS_DRAFT,
			},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"title"}},
		})
		require.NoError(t, err)
		assert.Equal(t, "Updated Title", resp.Content.Title)
		// Body should remain unchanged since it was not in the update mask.
		assert.Equal(t, "Some body text", resp.Content.Body)
	})

	t.Run("updates multiple fields", func(t *testing.T) {
		t.Parallel()
		spaceID := tc.createSpace(t)
		created, err := tc.clients.standard.CreateContent(tc.ctx, validCreateRequest(spaceID))
		require.NoError(t, err)

		resp, err := tc.clients.standard.UpdateContent(tc.ctx, &contentv1.UpdateContentRequest{
			Id: created.Content.Id,
			Content: &contentv1.Content{
				Title:  "New Title",
				Body:   "New body",
				Space:  &spacev1.SpaceRef{Id: spaceID},
				Status: contentv1.ContentStatus_CONTENT_STATUS_PUBLISHED,
				Tags:   []string{"updated"},
			},
			UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"body", "status", "tags"}},
		})
		require.NoError(t, err)
		assert.Equal(t, "New body", resp.Content.Body)
		assert.Equal(t, contentv1.ContentStatus_CONTENT_STATUS_PUBLISHED, resp.Content.Status)
		assert.Equal(t, []string{"updated"}, resp.Content.Tags)
		// Title should remain unchanged since it was not in the update mask.
		assert.Equal(t, "My Content", resp.Content.Title)
	})
}
