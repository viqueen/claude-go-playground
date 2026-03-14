package space

import (
	db "github.com/viqueen/claude-go-playground/grpc-backend/gen/db/collaboration"
	"github.com/viqueen/claude-go-playground/grpc-backend/internal/outbox/space/mappings"
)

// IndexName is the OpenSearch index name for spaces.
const IndexName = "spaces"

// EmbeddingDimension is the expected vector dimension, matching mappings/space.json.
const EmbeddingDimension = 1536

// IndexMapping is the OpenSearch mapping for the spaces index.
var IndexMapping = must(mappings.FS.ReadFile("space.json"))

func must(data []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return data
}

// EmbeddingField is the mapping field name for the vector embedding.
const EmbeddingField = "embedding"

// SpaceDocument represents the search document for a space.
// Fields match the mapping properties in mappings/space.json exactly.
type SpaceDocument struct {
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      int32     `json:"status"`
	Visibility  int32     `json:"visibility"`
	Embedding   []float32 `json:"embedding,omitempty"`
}

// toDocument maps a sqlc CollaborationSpace model to a search document.
func toDocument(entity *db.CollaborationSpace) SpaceDocument {
	return SpaceDocument{
		Key:         entity.Key,
		Name:        entity.Name,
		Description: entity.Description,
		Status:      entity.Status,
		Visibility:  entity.Visibility,
	}
}

// EmbeddingText returns the text to embed for this document.
func (d SpaceDocument) EmbeddingText() string {
	return d.Name + "\n" + d.Description
}
