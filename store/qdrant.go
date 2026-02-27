package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	agentsdk "github.com/cyberFlowTech/zapry-agents-sdk-go"
)

// QdrantStore implements agentsdk.EmbeddingStore using Qdrant's REST API.
type QdrantStore struct {
	baseURL    string
	collection string
	apiKey     string
	client     *http.Client
}

// QdrantConfig configures the Qdrant store.
type QdrantConfig struct {
	BaseURL    string // e.g. "http://localhost:6333"
	Collection string // collection name, default "memory"
	APIKey     string // optional API key
}

// NewQdrantStore creates an EmbeddingStore backed by Qdrant.
func NewQdrantStore(config QdrantConfig) *QdrantStore {
	if config.Collection == "" {
		config.Collection = "memory"
	}
	return &QdrantStore{
		baseURL:    strings.TrimRight(config.BaseURL, "/"),
		collection: config.Collection,
		apiKey:     config.APIKey,
		client:     &http.Client{},
	}
}

func (q *QdrantStore) url(path string) string {
	return fmt.Sprintf("%s/collections/%s%s", q.baseURL, q.collection, path)
}

func (q *QdrantStore) doRequest(ctx context.Context, method, url string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if q.apiKey != "" {
		req.Header.Set("api-key", q.apiKey)
	}

	resp, err := q.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("qdrant %s %s: %d %s", method, url, resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func (q *QdrantStore) Upsert(ctx context.Context, id string, embedding []float32, content string, metadata map[string]string) error {
	payload := map[string]interface{}{
		"content": content,
	}
	for k, v := range metadata {
		payload[k] = v
	}

	body := map[string]interface{}{
		"points": []map[string]interface{}{
			{
				"id":      id,
				"vector":  embedding,
				"payload": payload,
			},
		},
	}

	_, err := q.doRequest(ctx, "PUT", q.url("/points"), body)
	return err
}

func (q *QdrantStore) Search(ctx context.Context, query []float32, topK int, filter map[string]string) ([]agentsdk.MemoryHit, error) {
	body := map[string]interface{}{
		"vector":       query,
		"limit":        topK,
		"with_payload": true,
	}

	if len(filter) > 0 {
		must := make([]map[string]interface{}, 0, len(filter))
		for k, v := range filter {
			must = append(must, map[string]interface{}{
				"key":   k,
				"match": map[string]interface{}{"value": v},
			})
		}
		body["filter"] = map[string]interface{}{
			"must": must,
		}
	}

	respBody, err := q.doRequest(ctx, "POST", q.url("/points/search"), body)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result []struct {
			ID      interface{}            `json:"id"`
			Score   float32                `json:"score"`
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return nil, err
	}

	hits := make([]agentsdk.MemoryHit, 0, len(resp.Result))
	for _, r := range resp.Result {
		h := agentsdk.MemoryHit{
			ID:       fmt.Sprintf("%v", r.ID),
			Score:    r.Score,
			Metadata: make(map[string]string),
		}
		if content, ok := r.Payload["content"].(string); ok {
			h.Content = content
		}
		for k, v := range r.Payload {
			if k != "content" {
				h.Metadata[k] = fmt.Sprintf("%v", v)
			}
		}
		hits = append(hits, h)
	}
	return hits, nil
}

func (q *QdrantStore) Delete(ctx context.Context, ids []string) error {
	body := map[string]interface{}{
		"points": ids,
	}
	_, err := q.doRequest(ctx, "POST", q.url("/points/delete"), body)
	return err
}

func (q *QdrantStore) DeleteByMetadata(ctx context.Context, filter map[string]string) error {
	if len(filter) == 0 {
		return nil
	}
	must := make([]map[string]interface{}, 0, len(filter))
	for k, v := range filter {
		must = append(must, map[string]interface{}{
			"key":   k,
			"match": map[string]interface{}{"value": v},
		})
	}
	body := map[string]interface{}{
		"filter": map[string]interface{}{
			"must": must,
		},
	}
	_, err := q.doRequest(ctx, "POST", q.url("/points/delete"), body)
	return err
}

// Compile-time interface check.
var _ agentsdk.EmbeddingStore = (*QdrantStore)(nil)
