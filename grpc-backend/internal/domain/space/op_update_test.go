package space_test

import (
	"testing"

	uuid "github.com/gofrs/uuid/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	spacedomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/space"
)

func TestUpdate(t *testing.T) {
	svc, ctx := setupService(t)

	t.Run("not found", func(t *testing.T) {
		_, err := svc.Update(ctx, db.UpdateSpaceParams{
			ID:   uuid.Must(uuid.NewV4()),
			Name: pgtype.Text{String: "Updated", Valid: true},
		})
		assert.ErrorIs(t, err, spacedomain.ErrNotFound)
	})

	t.Run("success", func(t *testing.T) {
		created, err := svc.Create(ctx, db.CreateSpaceParams{
			Name: "Original", Key: "UPDT", Description: "original", Status: 1, Visibility: 1,
		})
		require.NoError(t, err)

		result, err := svc.Update(ctx, db.UpdateSpaceParams{
			ID:   created.ID,
			Name: pgtype.Text{String: "Updated Name", Valid: true},
		})
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", result.Name)
		assert.Equal(t, "UPDT", result.Key)
		assert.Equal(t, created.ID, result.ID)
	})
}
