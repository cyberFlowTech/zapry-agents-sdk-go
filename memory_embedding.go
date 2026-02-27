package agentsdk

import (
	"context"
)

// ──────────────────────────────────────────────
// Embedding Store — vector semantic retrieval layer
// ──────────────────────────────────────────────

// EmbedFunc generates a dense embedding vector for a text string.
// Callers wire this to their embedding provider (OpenAI, Cohere, local model, etc.).
type EmbedFunc func(ctx context.Context, text string) ([]float32, error)

// MemoryHit represents a single result from a vector similarity search.
type MemoryHit struct {
	ID       string            `json:"id"`
	Score    float32           `json:"score"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// EmbeddingStore is the pluggable interface for vector storage and retrieval.
// Implementations include pgvector, Qdrant, Milvus, Pinecone, ChromaDB, etc.
type EmbeddingStore interface {
	// Upsert inserts or updates a vector with associated content and metadata.
	Upsert(ctx context.Context, id string, embedding []float32, content string, metadata map[string]string) error

	// Search returns the top-K most similar vectors to the query embedding.
	// filter may be nil; implementations should support metadata-based filtering.
	Search(ctx context.Context, query []float32, topK int, filter map[string]string) ([]MemoryHit, error)

	// Delete removes vectors by their IDs.
	Delete(ctx context.Context, ids []string) error

	// DeleteByMetadata removes all vectors matching the metadata filter.
	// Useful for GDPR namespace-level deletion.
	DeleteByMetadata(ctx context.Context, filter map[string]string) error
}

// SemanticMemoryStore combines structured MemoryStore with vector EmbeddingStore,
// providing both exact-key and semantic-similarity access to long-term memories.
type SemanticMemoryStore struct {
	Structured MemoryStore
	Vectors    EmbeddingStore
	EmbedFn    EmbedFunc
}

// NewSemanticMemoryStore creates a dual-store that supports both KV and vector ops.
func NewSemanticMemoryStore(structured MemoryStore, vectors EmbeddingStore, embedFn EmbedFunc) *SemanticMemoryStore {
	return &SemanticMemoryStore{
		Structured: structured,
		Vectors:    vectors,
		EmbedFn:    embedFn,
	}
}

// IndexMemory generates an embedding for content and upserts it into the vector store.
func (s *SemanticMemoryStore) IndexMemory(ctx context.Context, id, content string, metadata map[string]string) error {
	if s.Vectors == nil || s.EmbedFn == nil {
		return nil
	}

	embedding, err := s.EmbedFn(ctx, content)
	if err != nil {
		return err
	}
	return s.Vectors.Upsert(ctx, id, embedding, content, metadata)
}

// SearchRelevant returns memories semantically similar to the query text.
func (s *SemanticMemoryStore) SearchRelevant(ctx context.Context, query string, topK int, filter map[string]string) ([]MemoryHit, error) {
	if s.Vectors == nil || s.EmbedFn == nil {
		return nil, nil
	}

	embedding, err := s.EmbedFn(ctx, query)
	if err != nil {
		return nil, err
	}
	return s.Vectors.Search(ctx, embedding, topK, filter)
}

// DeleteMemory removes a memory from both the vector store and structured store.
func (s *SemanticMemoryStore) DeleteMemory(ctx context.Context, id, namespace, key string) error {
	if s.Vectors != nil {
		if err := s.Vectors.Delete(ctx, []string{id}); err != nil {
			return err
		}
	}
	if namespace != "" && key != "" {
		return s.Structured.Delete(namespace, key)
	}
	return nil
}

// DeleteNamespaceVectors removes all vectors for a namespace via metadata filter.
func (s *SemanticMemoryStore) DeleteNamespaceVectors(ctx context.Context, namespace string) error {
	if s.Vectors == nil {
		return nil
	}
	return s.Vectors.DeleteByMetadata(ctx, map[string]string{"namespace": namespace})
}
