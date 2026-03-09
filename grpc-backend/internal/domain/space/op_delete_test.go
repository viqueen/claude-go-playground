package space_test

import (
	"testing"

	uuid "github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	spacedomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/space"
)

func TestDelete(t *testing.T) {
	svc, ctx := setupService(t)

	t.Run("not found", func(t *testing.T) {
		err := svc.Delete(ctx, uuid.Must(uuid.NewV4()))
		assert.ErrorIs(t, err, spacedomain.ErrNotFound)
	})

	t.Run("success", func(t *testing.T) {
		created, err := svc.Create(ctx, db.CreateSpaceParams{
			Name: "To Delete", Key: "DEL", Description: "", Status: 1, Visibility: 1,
		})
		require.NoError(t, err)

		err = svc.Delete(ctx, created.ID)
		require.NoError(t, err)

		_, err = svc.Get(ctx, created.ID)
		assert.ErrorIs(t, err, spacedomain.ErrNotFound)
	})
}
