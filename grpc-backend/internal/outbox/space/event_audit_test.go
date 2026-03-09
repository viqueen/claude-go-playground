package space_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	spacedomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/space"
	spaceoutbox "github.com/viqueen/claude-go-playground/grpc-backend/internal/outbox/space"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
)

func TestNewAuditArgs(t *testing.T) {
	t.Run("constructs args from event", func(t *testing.T) {
		event := outbox.Event{
			Type: spacedomain.EventCreated,
			ID:   "test-space-id",
			Data: nil,
		}

		args := spaceoutbox.NewAuditArgs(event)
		assert.Equal(t, spacedomain.EventCreated, args.EventType)
		assert.Equal(t, "test-space-id", args.SpaceID)
		assert.Equal(t, "space.audit", args.Kind())
	})
}
