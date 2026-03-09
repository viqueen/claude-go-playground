package space_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	spacedomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/space"
)

func TestCreate(t *testing.T) {
	svc, ctx := setupService(t)

	t.Run("already exists — duplicate key", func(t *testing.T) {
		_, err := svc.Create(ctx, db.CreateSpaceParams{
			Name:        "First Space",
			Key:         "DUPKEY",
			Description: "first",
			Status:      1,
			Visibility:  1,
		})
		require.NoError(t, err)

		_, err = svc.Create(ctx, db.CreateSpaceParams{
			Name:        "Second Space",
			Key:         "DUPKEY",
			Description: "second",
			Status:      1,
			Visibility:  1,
		})
		assert.ErrorIs(t, err, spacedomain.ErrAlreadyExists)
	})

	t.Run("success", func(t *testing.T) {
		result, err := svc.Create(ctx, validCreateParams())
		require.NoError(t, err)
		assert.Equal(t, "Test Space", result.Name)
		assert.Equal(t, "TEST", result.Key)
		assert.Equal(t, "A test space", result.Description)
		assert.Equal(t, int32(1), result.Status)
		assert.Equal(t, int32(1), result.Visibility)
		assert.NotEmpty(t, result.ID)
		assert.False(t, result.CreatedAt.IsZero())
		assert.False(t, result.UpdatedAt.IsZero())
	})
}
