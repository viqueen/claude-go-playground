package content_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	contentdomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/content"
	contentoutbox "github.com/viqueen/claude-go-playground/grpc-backend/internal/outbox/content"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
)

func TestNewIndexArgs(t *testing.T) {
	t.Run("constructs args from event", func(t *testing.T) {
		event := outbox.Event{
			Type: contentdomain.EventUpdated,
			ID:   "test-content-id",
			Data: nil,
		}

		args := contentoutbox.NewIndexArgs(event)
		assert.Equal(t, contentdomain.EventUpdated, args.EventType)
		assert.Equal(t, "test-content-id", args.ContentID)
		assert.Equal(t, "content.index", args.Kind())
	})
}
