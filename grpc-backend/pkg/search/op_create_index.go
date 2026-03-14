package search

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/rs/zerolog/log"
)

func (c *openSearchClient) CreateIndexIfNotExists(ctx context.Context, index string, mapping []byte) error {
	// Check if index exists
	resp, err := c.client.Indices.Exists(ctx, opensearchapi.IndicesExistsReq{
		Indices: []string{index},
	})
	if err != nil {
		return fmt.Errorf("search: check index exists: %w", err)
	}
	defer resp.Body.Close()
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
	defer createResp.Inspect().Response.Body.Close()
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
