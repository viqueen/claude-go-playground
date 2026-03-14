package search

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gofrs/uuid/v5"
	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
)

// Filter represents an exact-match constraint on a keyword or integer field.
type Filter struct {
	Field string
	Value any
}

// Match represents a full-text search on a text field.
type Match struct {
	Field string
	Query string
}

// Vector represents a k-NN vector search on a knn_vector field.
type Vector struct {
	Field  string
	Values []float32
	K      int
}

// Criteria defines a typed search query.
type Criteria struct {
	Filters   []Filter
	Matches   []Match
	Vector    *Vector
	PageSize  int32
	PageToken string
}

// Page represents a paginated set of search results.
type Page struct {
	Hits          []Hit
	NextPageToken string
}

// Hit represents a single search result with its raw JSON source.
type Hit struct {
	ID     uuid.UUID
	Score  float32
	Source json.RawMessage
}

// Search defines the interface for indexing, deleting, and querying documents.
type Search interface {
	// Index indexes or updates a document in the given index.
	Index(ctx context.Context, index string, id uuid.UUID, document any) error
	// Delete removes a document from the given index.
	Delete(ctx context.Context, index string, id uuid.UUID) error
	// Find searches an index using typed criteria and returns a paginated result.
	Find(ctx context.Context, index string, criteria Criteria) (*Page, error)
	// CreateIndexIfNotExists ensures an index exists with the given mapping.
	CreateIndexIfNotExists(ctx context.Context, index string, mapping []byte) error
}

// New creates a new Search client backed by OpenSearch.
func New(address string) (Search, error) {
	client, err := opensearchapi.NewClient(opensearchapi.Config{
		Client: opensearch.Config{
			Addresses: []string{address},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("search: create client: %w", err)
	}
	return &openSearchClient{client: client}, nil
}

type openSearchClient struct {
	client *opensearchapi.Client
}
