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
	defer resp.Inspect().Response.Body.Close()
	if resp.Inspect().Response.StatusCode >= http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Inspect().Response.Body)
		log.Ctx(ctx).Error().Str("index", index).Str("id", id.String()).Str("response", string(respBody)).Msg("search: index document error response")
		return fmt.Errorf("search: index document: status %d", resp.Inspect().Response.StatusCode)
	}
	return nil
}
