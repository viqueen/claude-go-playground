package space_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	spacedomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/space"
	spaceoutbox "github.com/viqueen/claude-go-playground/grpc-backend/internal/outbox/space"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
)

func TestNewIndexArgs(t *testing.T) {
	t.Run("constructs args from event", func(t *testing.T) {
		event := outbox.Event{
			Type: spacedomain.EventUpdated,
			ID:   "test-space-id",
			Data: nil,
		}

		args := spaceoutbox.NewIndexArgs(event)
		assert.Equal(t, spacedomain.EventUpdated, args.EventType)
		assert.Equal(t, "test-space-id", args.SpaceID)
		assert.Equal(t, "space.index", args.Kind())
	})
}
