package space_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/gofrs/uuid/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	spacedomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/space"
	spaceoutbox "github.com/viqueen/claude-go-playground/grpc-backend/internal/outbox/space"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/outbox"
	"github.com/viqueen/claude-go-playground/grpc-backend/pkg/search"
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

// --- mocks ---

type mockEmbedder struct {
	embedding []float32
	err       error
}

func (m *mockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return m.embedding, m.err
}

type mockSearch struct {
	indexErr  error
	deleteErr error
	indexed   bool
	deleted   bool
}

func (m *mockSearch) Index(_ context.Context, _ string, _ uuid.UUID, _ any) error {
	m.indexed = true
	return m.indexErr
}

func (m *mockSearch) Delete(_ context.Context, _ string, _ uuid.UUID) error {
	m.deleted = true
	return m.deleteErr
}

func (m *mockSearch) Find(_ context.Context, _ string, _ search.Criteria) (*search.Page, error) {
	return nil, nil
}

func (m *mockSearch) CreateIndexIfNotExists(_ context.Context, _ string, _ []byte) error {
	return nil
}

type mockStore struct {
	space db.CollaborationSpace
	err   error
}

func (m *mockStore) GetSpace(_ context.Context, _ uuid.UUID) (db.CollaborationSpace, error) {
	return m.space, m.err
}

// --- tests ---

func TestIndexWorkerWork(t *testing.T) {
	spaceID := uuid.Must(uuid.NewV4())
	validEmbedding := make([]float32, spaceoutbox.EmbeddingDimension)
	testSpace := db.CollaborationSpace{Key: "TST", Name: "Test", Description: "desc"}

	t.Run("indexes on create event", func(t *testing.T) {
		s := &mockSearch{}
		worker := spaceoutbox.NewIndexWorker(spaceoutbox.IndexDependencies{
			Search:   s,
			Embedder: &mockEmbedder{embedding: validEmbedding},
			Queries:  &mockStore{space: testSpace},
		})
		err := worker.Work(context.Background(), &river.Job[spaceoutbox.IndexArgs]{
			JobRow: &rivertype.JobRow{},
			Args:   spaceoutbox.IndexArgs{EventType: spacedomain.EventCreated, SpaceID: spaceID.String()},
		})
		require.NoError(t, err)
		assert.True(t, s.indexed)
	})

	t.Run("indexes on update event", func(t *testing.T) {
		s := &mockSearch{}
		worker := spaceoutbox.NewIndexWorker(spaceoutbox.IndexDependencies{
			Search:   s,
			Embedder: &mockEmbedder{embedding: validEmbedding},
			Queries:  &mockStore{space: testSpace},
		})
		err := worker.Work(context.Background(), &river.Job[spaceoutbox.IndexArgs]{
			JobRow: &rivertype.JobRow{},
			Args:   spaceoutbox.IndexArgs{EventType: spacedomain.EventUpdated, SpaceID: spaceID.String()},
		})
		require.NoError(t, err)
		assert.True(t, s.indexed)
	})

	t.Run("deletes on delete event", func(t *testing.T) {
		s := &mockSearch{}
		worker := spaceoutbox.NewIndexWorker(spaceoutbox.IndexDependencies{
			Search:   s,
			Embedder: &mockEmbedder{},
			Queries:  &mockStore{space: testSpace},
		})
		err := worker.Work(context.Background(), &river.Job[spaceoutbox.IndexArgs]{
			JobRow: &rivertype.JobRow{},
			Args:   spaceoutbox.IndexArgs{EventType: spacedomain.EventDeleted, SpaceID: spaceID.String()},
		})
		require.NoError(t, err)
		assert.True(t, s.deleted)
	})

	t.Run("returns error for invalid UUID", func(t *testing.T) {
		worker := spaceoutbox.NewIndexWorker(spaceoutbox.IndexDependencies{
			Search:   &mockSearch{},
			Embedder: &mockEmbedder{},
			Queries:  &mockStore{space: testSpace},
		})
		err := worker.Work(context.Background(), &river.Job[spaceoutbox.IndexArgs]{
			JobRow: &rivertype.JobRow{},
			Args:   spaceoutbox.IndexArgs{EventType: spacedomain.EventCreated, SpaceID: "not-a-uuid"},
		})
		require.Error(t, err)
	})

	t.Run("returns error when embedder fails", func(t *testing.T) {
		worker := spaceoutbox.NewIndexWorker(spaceoutbox.IndexDependencies{
			Search:   &mockSearch{},
			Embedder: &mockEmbedder{err: fmt.Errorf("embedder down")},
			Queries:  &mockStore{space: testSpace},
		})
		err := worker.Work(context.Background(), &river.Job[spaceoutbox.IndexArgs]{
			JobRow: &rivertype.JobRow{},
			Args:   spaceoutbox.IndexArgs{EventType: spacedomain.EventCreated, SpaceID: spaceID.String()},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "embedder down")
	})

	t.Run("returns error when embedding dimension mismatches", func(t *testing.T) {
		worker := spaceoutbox.NewIndexWorker(spaceoutbox.IndexDependencies{
			Search:   &mockSearch{},
			Embedder: &mockEmbedder{embedding: make([]float32, 768)},
			Queries:  &mockStore{space: testSpace},
		})
		err := worker.Work(context.Background(), &river.Job[spaceoutbox.IndexArgs]{
			JobRow: &rivertype.JobRow{},
			Args:   spaceoutbox.IndexArgs{EventType: spacedomain.EventCreated, SpaceID: spaceID.String()},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "dimension mismatch")
	})

	t.Run("returns error when search index fails", func(t *testing.T) {
		worker := spaceoutbox.NewIndexWorker(spaceoutbox.IndexDependencies{
			Search:   &mockSearch{indexErr: fmt.Errorf("search unavailable")},
			Embedder: &mockEmbedder{embedding: validEmbedding},
			Queries:  &mockStore{space: testSpace},
		})
		err := worker.Work(context.Background(), &river.Job[spaceoutbox.IndexArgs]{
			JobRow: &rivertype.JobRow{},
			Args:   spaceoutbox.IndexArgs{EventType: spacedomain.EventCreated, SpaceID: spaceID.String()},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "search unavailable")
	})

	t.Run("returns error when search delete fails", func(t *testing.T) {
		worker := spaceoutbox.NewIndexWorker(spaceoutbox.IndexDependencies{
			Search:   &mockSearch{deleteErr: fmt.Errorf("delete failed")},
			Embedder: &mockEmbedder{},
			Queries:  &mockStore{space: testSpace},
		})
		err := worker.Work(context.Background(), &river.Job[spaceoutbox.IndexArgs]{
			JobRow: &rivertype.JobRow{},
			Args:   spaceoutbox.IndexArgs{EventType: spacedomain.EventDeleted, SpaceID: spaceID.String()},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "delete failed")
	})

	t.Run("returns nil for unknown event type", func(t *testing.T) {
		s := &mockSearch{}
		worker := spaceoutbox.NewIndexWorker(spaceoutbox.IndexDependencies{
			Search:   s,
			Embedder: &mockEmbedder{},
			Queries:  &mockStore{space: testSpace},
		})
		err := worker.Work(context.Background(), &river.Job[spaceoutbox.IndexArgs]{
			JobRow: &rivertype.JobRow{},
			Args:   spaceoutbox.IndexArgs{EventType: "space.unknown", SpaceID: spaceID.String()},
		})
		require.NoError(t, err)
		assert.False(t, s.indexed)
		assert.False(t, s.deleted)
	})

	t.Run("returns error when DB query fails", func(t *testing.T) {
		worker := spaceoutbox.NewIndexWorker(spaceoutbox.IndexDependencies{
			Search:   &mockSearch{},
			Embedder: &mockEmbedder{},
			Queries:  &mockStore{err: fmt.Errorf("db connection lost")},
		})
		err := worker.Work(context.Background(), &river.Job[spaceoutbox.IndexArgs]{
			JobRow: &rivertype.JobRow{},
			Args:   spaceoutbox.IndexArgs{EventType: spacedomain.EventCreated, SpaceID: spaceID.String()},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db connection lost")
	})
}
