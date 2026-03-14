package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gofrs/uuid/v5"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/rs/zerolog/log"
)

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
	defer resp.Inspect().Response.Body.Close()
	if resp.Inspect().Response.StatusCode >= http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Inspect().Response.Body)
		return nil, fmt.Errorf("search: find: status %d: %s", resp.Inspect().Response.StatusCode, string(respBody))
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
	if int32(len(resp.Hits.Hits)) == pageSize {
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
