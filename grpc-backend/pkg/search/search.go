package search

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gofrs/uuid/v5"
	"github.com/opensearch-project/opensearch-go/v4"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/rs/zerolog/log"
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

func (c *openSearchClient) Index(ctx context.Context, index string, id uuid.UUID, document any) error {
	body, err := json.Marshal(document)
	if err != nil {
		return fmt.Errorf("search: marshal document: %w", err)
	}

	resp, err := c.client.Index(ctx, opensearchapi.IndexReq{
		Index:      index,
		DocumentID: id.String(),
		Body:       bytes.NewReader(body),
	})
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("index", index).Str("id", id.String()).Msg("search: index document failed")
		return fmt.Errorf("search: index document: %w", err)
	}
	if resp.Inspect().Response.StatusCode >= http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Inspect().Response.Body)
		log.Ctx(ctx).Error().Str("index", index).Str("id", id.String()).Str("response", string(respBody)).Msg("search: index document error response")
		return fmt.Errorf("search: index document: status %d", resp.Inspect().Response.StatusCode)
	}
	return nil
}

func (c *openSearchClient) Delete(ctx context.Context, index string, id uuid.UUID) error {
	resp, err := c.client.Document.Delete(ctx, opensearchapi.DocumentDeleteReq{
		Index:      index,
		DocumentID: id.String(),
	})
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("index", index).Str("id", id.String()).Msg("search: delete document failed")
		return fmt.Errorf("search: delete document: %w", err)
	}
	if resp.Inspect().Response.StatusCode >= http.StatusBadRequest {
		// 404 on delete is acceptable — document may already be gone
		if resp.Inspect().Response.StatusCode == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("search: delete document: status %d", resp.Inspect().Response.StatusCode)
	}
	return nil
}

func (c *openSearchClient) Find(ctx context.Context, index string, criteria Criteria) (*Page, error) {
	query := buildQuery(criteria)

	pageSize := criteria.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	searchBody := map[string]any{
		"size": pageSize,
	}

	if query != nil {
		searchBody["query"] = query
	}

	// Consistent sort for pagination
	searchBody["sort"] = []map[string]any{
		{"_score": map[string]string{"order": "desc"}},
		{"_id": map[string]string{"order": "asc"}},
	}

	// Decode search_after from page token
	if criteria.PageToken != "" {
		searchAfter, err := decodeSearchAfter(criteria.PageToken)
		if err != nil {
			return nil, fmt.Errorf("search: invalid page token: %w", err)
		}
		searchBody["search_after"] = searchAfter
	}

	body, err := json.Marshal(searchBody)
	if err != nil {
		return nil, fmt.Errorf("search: marshal query: %w", err)
	}

	resp, err := c.client.Search(ctx, &opensearchapi.SearchReq{
		Indices: []string{index},
		Body:    bytes.NewReader(body),
	})
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("index", index).Msg("search: find failed")
		return nil, fmt.Errorf("search: find: %w", err)
	}

	var hits []Hit
	for _, h := range resp.Hits.Hits {
		id, err := uuid.FromString(h.ID)
		if err != nil {
			log.Ctx(ctx).Warn().Str("id", h.ID).Msg("search: skipping hit with invalid UUID")
			continue
		}
		hits = append(hits, Hit{
			ID:     id,
			Score:  h.Score,
			Source: h.Source,
		})
	}

	var nextPageToken string
	if int32(len(hits)) == pageSize && len(resp.Hits.Hits) > 0 {
		lastHit := resp.Hits.Hits[len(resp.Hits.Hits)-1]
		if lastHit.Sort != nil {
			nextPageToken = encodeSearchAfter(lastHit.Sort)
		}
	}

	return &Page{
		Hits:          hits,
		NextPageToken: nextPageToken,
	}, nil
}

func (c *openSearchClient) CreateIndexIfNotExists(ctx context.Context, index string, mapping []byte) error {
	// Check if index exists
	resp, err := c.client.Indices.Exists(ctx, opensearchapi.IndicesExistsReq{
		Indices: []string{index},
	})
	if err != nil {
		return fmt.Errorf("search: check index exists: %w", err)
	}
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	// Create index with mapping
	createResp, err := c.client.Indices.Create(ctx, opensearchapi.IndicesCreateReq{
		Index: index,
		Body:  bytes.NewReader(mapping),
	})
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("index", index).Msg("search: create index failed")
		return fmt.Errorf("search: create index: %w", err)
	}
	if createResp.Inspect().Response.StatusCode >= http.StatusBadRequest {
		respBody, _ := io.ReadAll(createResp.Inspect().Response.Body)
		// Ignore "resource_already_exists_exception" in case of a race
		if strings.Contains(string(respBody), "resource_already_exists_exception") {
			return nil
		}
		return fmt.Errorf("search: create index: status %d: %s", createResp.Inspect().Response.StatusCode, string(respBody))
	}

	log.Ctx(ctx).Info().Str("index", index).Msg("search: index created")
	return nil
}

// buildQuery translates Criteria into an OpenSearch query.
func buildQuery(criteria Criteria) map[string]any {
	hasFilters := len(criteria.Filters) > 0
	hasMatches := len(criteria.Matches) > 0
	hasVector := criteria.Vector != nil

	if !hasFilters && !hasMatches && !hasVector {
		return map[string]any{"match_all": map[string]any{}}
	}

	// When we have vector + matches, use hybrid query
	if hasVector && hasMatches {
		return buildHybridQuery(criteria)
	}

	// When we have only vector, use knn query
	if hasVector && !hasMatches {
		knnField := map[string]any{
			"vector": criteria.Vector.Values,
			"k":      criteria.Vector.K,
		}
		if hasFilters {
			knnField["filter"] = map[string]any{
				"bool": map[string]any{
					"filter": buildFilterClauses(criteria.Filters),
				},
			}
		}
		return map[string]any{
			"knn": map[string]any{
				criteria.Vector.Field: knnField,
			},
		}
	}

	// Standard bool query for filters + matches
	boolQuery := map[string]any{}
	if hasFilters {
		boolQuery["filter"] = buildFilterClauses(criteria.Filters)
	}
	if hasMatches {
		boolQuery["must"] = buildMatchClauses(criteria.Matches)
	}

	return map[string]any{"bool": boolQuery}
}

// buildHybridQuery creates an OpenSearch hybrid query combining text and vector search.
func buildHybridQuery(criteria Criteria) map[string]any {
	boolQuery := map[string]any{}
	if len(criteria.Filters) > 0 {
		boolQuery["filter"] = buildFilterClauses(criteria.Filters)
	}
	boolQuery["must"] = buildMatchClauses(criteria.Matches)

	return map[string]any{
		"hybrid": map[string]any{
			"queries": []map[string]any{
				{"bool": boolQuery},
				{
					"knn": map[string]any{
						criteria.Vector.Field: map[string]any{
							"vector": criteria.Vector.Values,
							"k":      criteria.Vector.K,
						},
					},
				},
			},
		},
	}
}

func buildFilterClauses(filters []Filter) []map[string]any {
	clauses := make([]map[string]any, len(filters))
	for i, f := range filters {
		clauses[i] = map[string]any{
			"term": map[string]any{
				f.Field: f.Value,
			},
		}
	}
	return clauses
}

func buildMatchClauses(matches []Match) []map[string]any {
	clauses := make([]map[string]any, len(matches))
	for i, m := range matches {
		clauses[i] = map[string]any{
			"match": map[string]any{
				m.Field: m.Query,
			},
		}
	}
	return clauses
}

func encodeSearchAfter(sortValues []any) string {
	data, _ := json.Marshal(sortValues)
	return base64.StdEncoding.EncodeToString(data)
}

func decodeSearchAfter(token string) ([]any, error) {
	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	var sortValues []any
	if err := json.Unmarshal(data, &sortValues); err != nil {
		return nil, err
	}
	return sortValues, nil
}
