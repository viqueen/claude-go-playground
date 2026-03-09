package space_test

import (
	"testing"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestList(t *testing.T) {
	svc, ctx := setupService(t)

	t.Run("empty list", func(t *testing.T) {
		spaces, nextToken, err := svc.List(ctx, 10, "")
		require.NoError(t, err)
		assert.Empty(t, spaces)
		assert.Empty(t, nextToken)
	})

	t.Run("success — returns created spaces", func(t *testing.T) {
		_, err := svc.Create(ctx, db.CreateSpaceParams{
			Name: "Space A", Key: "LISTA", Description: "a", Status: 1, Visibility: 1,
		})
		require.NoError(t, err)

		_, err = svc.Create(ctx, db.CreateSpaceParams{
			Name: "Space B", Key: "LISTB", Description: "b", Status: 1, Visibility: 1,
		})
		require.NoError(t, err)

		spaces, _, err := svc.List(ctx, 10, "")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(spaces), 2)
	})

	t.Run("success — pagination", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			_, err := svc.Create(ctx, db.CreateSpaceParams{
				Name: "Page Space", Key: "PAGE" + string(rune('A'+i)), Description: "", Status: 1, Visibility: 1,
			})
			require.NoError(t, err)
		}

		spaces, nextToken, err := svc.List(ctx, 1, "")
		require.NoError(t, err)
		assert.Len(t, spaces, 1)
		assert.NotEmpty(t, nextToken)

		spaces2, _, err := svc.List(ctx, 1, nextToken)
		require.NoError(t, err)
		assert.Len(t, spaces2, 1)
		assert.NotEqual(t, spaces[0].ID, spaces2[0].ID)
	})
}
