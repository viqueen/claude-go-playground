package space

// Event type constants for outbox events emitted by the space domain.
const (
	EventCreated        = "space.created"
	EventUpdated        = "space.updated"
	EventDeleted        = "space.deleted"
	EventContentDeleted = "space.content_deleted"
)
