package search

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gofrs/uuid/v5"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/rs/zerolog/log"
)

func (c *openSearchClient) Delete(ctx context.Context, index string, id uuid.UUID) error {
	resp, err := c.client.Document.Delete(ctx, opensearchapi.DocumentDeleteReq{
		Index:      index,
		DocumentID: id.String(),
	})
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("index", index).Str("id", id.String()).Msg("search: delete document failed")
		return fmt.Errorf("search: delete document: %w", err)
	}
	defer resp.Inspect().Response.Body.Close()
	if resp.Inspect().Response.StatusCode >= http.StatusBadRequest {
		// 404 on delete is acceptable — document may already be gone
		if resp.Inspect().Response.StatusCode == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("search: delete document: status %d", resp.Inspect().Response.StatusCode)
	}
	return nil
}
