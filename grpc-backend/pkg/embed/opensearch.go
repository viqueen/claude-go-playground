package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// NewOpenSearch creates an Embedder backed by OpenSearch's built-in ML plugin.
// The model must be deployed via the ML commons plugin at _plugins/_ml/models/<modelID>/_predict.
func NewOpenSearch(address string, modelID string) (Embedder, error) {
	return &openSearchEmbedder{
		client:  &http.Client{},
		address: address,
		modelID: modelID,
	}, nil
}

type openSearchEmbedder struct {
	client  *http.Client
	address string
	modelID string
}

type predictRequest struct {
	TextDocs []string `json:"text_docs"`
}

type predictResponse struct {
	InferenceResults []inferenceResult `json:"inference_results"`
}

type inferenceResult struct {
	Output []inferenceOutput `json:"output"`
}

type inferenceOutput struct {
	Data []float32 `json:"data"`
}

func (e *openSearchEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(predictRequest{TextDocs: []string{text}})
	if err != nil {
		return nil, fmt.Errorf("embed: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/_plugins/_ml/models/%s/_predict", e.address, e.modelID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embed: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed: send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embed: unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	var result predictResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("embed: decode response: %w", err)
	}

	if len(result.InferenceResults) == 0 || len(result.InferenceResults[0].Output) == 0 {
		return nil, fmt.Errorf("embed: empty inference results")
	}

	return result.InferenceResults[0].Output[0].Data, nil
}
