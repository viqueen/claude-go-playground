package space

import (
	"context"
	"fmt"
	"testing"

	"github.com/gofrs/uuid/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	spacedomain "github.com/viqueen/claude-go-playground/grpc-backend/internal/domain/space"
)

type testMocks struct {
	search   *MockSearch
	embedder *MockEmbedder
	store    *MockIndexStore
}

type workTestCase struct {
	args              IndexArgs
	expectedMockCalls func(mocks testMocks)
	expectedError     error
}

func TestIndexWorkerWork(t *testing.T) {
	spaceID := uuid.Must(uuid.NewV4())
	validEmbedding := make([]float32, EmbeddingDimension)
	testSpace := db.CollaborationSpace{Key: "TST", Name: "Test", Description: "desc"}

	tests := map[string]workTestCase{
		"returns error for invalid UUID": {
			args:              IndexArgs{EventType: spacedomain.EventCreated, SpaceID: "not-a-uuid"},
			expectedMockCalls: func(_ testMocks) {},
			expectedError:     fmt.Errorf("uuid: incorrect UUID length 10 in string \"not-a-uuid\""),
		},
		"returns error when DB query fails on create": {
			args: IndexArgs{EventType: spacedomain.EventCreated, SpaceID: spaceID.String()},
			expectedMockCalls: func(m testMocks) {
				m.store.EXPECT().GetSpace(mock.Anything, spaceID).Return(db.CollaborationSpace{}, fmt.Errorf("db connection lost"))
			},
			expectedError: fmt.Errorf("db connection lost"),
		},
		"returns error when embedder fails": {
			args: IndexArgs{EventType: spacedomain.EventCreated, SpaceID: spaceID.String()},
			expectedMockCalls: func(m testMocks) {
				m.store.EXPECT().GetSpace(mock.Anything, spaceID).Return(testSpace, nil)
				m.embedder.EXPECT().Embed(mock.Anything, "Test\ndesc").Return(nil, fmt.Errorf("embedder down"))
			},
			expectedError: fmt.Errorf("embedder down"),
		},
		"returns error when embedding dimension mismatches": {
			args: IndexArgs{EventType: spacedomain.EventCreated, SpaceID: spaceID.String()},
			expectedMockCalls: func(m testMocks) {
				m.store.EXPECT().GetSpace(mock.Anything, spaceID).Return(testSpace, nil)
				m.embedder.EXPECT().Embed(mock.Anything, "Test\ndesc").Return(make([]float32, 768), nil)
			},
			expectedError: fmt.Errorf("search: embedding dimension mismatch: got 768, want 1536"),
		},
		"returns error when search index fails": {
			args: IndexArgs{EventType: spacedomain.EventCreated, SpaceID: spaceID.String()},
			expectedMockCalls: func(m testMocks) {
				m.store.EXPECT().GetSpace(mock.Anything, spaceID).Return(testSpace, nil)
				m.embedder.EXPECT().Embed(mock.Anything, "Test\ndesc").Return(validEmbedding, nil)
				m.search.EXPECT().Index(mock.Anything, IndexName, spaceID, mock.Anything).Return(fmt.Errorf("search unavailable"))
			},
			expectedError: fmt.Errorf("search unavailable"),
		},
		"indexes on create event": {
			args: IndexArgs{EventType: spacedomain.EventCreated, SpaceID: spaceID.String()},
			expectedMockCalls: func(m testMocks) {
				m.store.EXPECT().GetSpace(mock.Anything, spaceID).Return(testSpace, nil)
				m.embedder.EXPECT().Embed(mock.Anything, "Test\ndesc").Return(validEmbedding, nil)
				m.search.EXPECT().Index(mock.Anything, IndexName, spaceID, mock.Anything).Return(nil)
			},
			expectedError: nil,
		},
		"indexes on update event": {
			args: IndexArgs{EventType: spacedomain.EventUpdated, SpaceID: spaceID.String()},
			expectedMockCalls: func(m testMocks) {
				m.store.EXPECT().GetSpace(mock.Anything, spaceID).Return(testSpace, nil)
				m.embedder.EXPECT().Embed(mock.Anything, "Test\ndesc").Return(validEmbedding, nil)
				m.search.EXPECT().Index(mock.Anything, IndexName, spaceID, mock.Anything).Return(nil)
			},
			expectedError: nil,
		},
		"returns error when search delete fails": {
			args: IndexArgs{EventType: spacedomain.EventDeleted, SpaceID: spaceID.String()},
			expectedMockCalls: func(m testMocks) {
				m.search.EXPECT().Delete(mock.Anything, IndexName, spaceID).Return(fmt.Errorf("delete failed"))
			},
			expectedError: fmt.Errorf("delete failed"),
		},
		"deletes on delete event": {
			args: IndexArgs{EventType: spacedomain.EventDeleted, SpaceID: spaceID.String()},
			expectedMockCalls: func(m testMocks) {
				m.search.EXPECT().Delete(mock.Anything, IndexName, spaceID).Return(nil)
			},
			expectedError: nil,
		},
		"returns nil for unknown event type": {
			args:              IndexArgs{EventType: "space.unknown", SpaceID: spaceID.String()},
			expectedMockCalls: func(_ testMocks) {},
			expectedError:     nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// setup mocks
			searchMock := NewMockSearch(t)
			embedderMock := NewMockEmbedder(t)
			storeMock := NewMockIndexStore(t)

			// set expectations
			tc.expectedMockCalls(testMocks{
				search:   searchMock,
				embedder: embedderMock,
				store:    storeMock,
			})

			// setup invocation
			worker := NewIndexWorker(IndexDependencies{
				Search:   searchMock,
				Embedder: embedderMock,
				Queries:  storeMock,
			})

			// invoke
			err := worker.Work(context.Background(), &river.Job[IndexArgs]{
				JobRow: &rivertype.JobRow{},
				Args:   tc.args,
			})

			// assert
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
