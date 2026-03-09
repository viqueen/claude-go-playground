package outbox

import "context"

// Event represents a domain event to be processed asynchronously.
type Event struct {
	Type string
	ID   string
	Data any
}

// Outbox emits domain events within a transaction.
// Generic over the transaction type to avoid unsafe casts while keeping pkg dependency-free.
type Outbox[T any] interface {
	Emit(ctx context.Context, tx T, events ...Event) error
}
