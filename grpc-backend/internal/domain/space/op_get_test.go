package space_test

import (
	"testing"

	uuid "github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	spacedomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/space"
)

func TestGet(t *testing.T) {
	svc, ctx := setupService(t)

	t.Run("not found", func(t *testing.T) {
		_, err := svc.Get(ctx, uuid.Must(uuid.NewV4()))
		assert.ErrorIs(t, err, spacedomain.ErrNotFound)
	})

	t.Run("success", func(t *testing.T) {
		created, err := svc.Create(ctx, validCreateParams())
		require.NoError(t, err)

		result, err := svc.Get(ctx, created.ID)
		require.NoError(t, err)
		assert.Equal(t, created.ID, result.ID)
		assert.Equal(t, created.Name, result.Name)
		assert.Equal(t, created.Key, result.Key)
	})
}
