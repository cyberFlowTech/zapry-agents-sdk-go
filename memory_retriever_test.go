package agentsdk

import (
	"testing"
	"time"
)

func TestTokenBudgetConfig_Defaults(t *testing.T) {
	cfg := DefaultTokenBudgetConfig()
	if cfg.MemoryBudget() != 1600 {
		t.Errorf("expected 1600, got %d", cfg.MemoryBudget())
	}
	if cfg.HistoryBudget() != 4000 {
		t.Errorf("expected 4000, got %d", cfg.HistoryBudget())
	}
	if cfg.SystemBudget() != 2400 {
		t.Errorf("expected 2400, got %d", cfg.SystemBudget())
	}
}

func TestMemoryRetriever_StructuredOnly(t *testing.T) {
	store := NewInMemoryMemoryStore()
	lt := NewLongTermMemory(store, "test:user1", 5*time.Minute)
	lt.Update(map[string]interface{}{
		"basic_info": map[string]interface{}{"age": float64(25), "location": "Shanghai"},
		"interests":  []interface{}{"coding", "music"},
	})

	retriever := NewMemoryRetriever(MemoryRetrieverOptions{
		Structured: lt,
		Budget:     DefaultTokenBudgetConfig(),
	})

	result, err := retriever.Retrieve(nil, "what are user's interests", 5)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text == "" {
		t.Error("expected non-empty text from structured memory")
	}
	if result.TokensUsed <= 0 {
		t.Error("expected positive token count")
	}
}

func TestMemoryRetriever_TypedMemory(t *testing.T) {
	store := NewInMemoryMemoryStore()
	typed := NewTypedMemoryStore(store, "test:user1")
	typed.Add(TypedMemory{ID: "s1", Type: MemoryTypeSemantic, Content: "User likes Go", Score: 0.8})
	typed.Add(TypedMemory{ID: "p1", Type: MemoryTypeProcedural, Content: "Reply concisely", Score: 0.9})

	retriever := NewMemoryRetriever(MemoryRetrieverOptions{
		Typed:  typed,
		Budget: DefaultTokenBudgetConfig(),
	})

	result, err := retriever.Retrieve(nil, "tell me about the user", 5)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text == "" {
		t.Error("expected text from typed memories")
	}
}

func TestMemoryRetriever_TruncateHistory(t *testing.T) {
	retriever := NewMemoryRetriever(MemoryRetrieverOptions{
		Budget: TokenBudgetConfig{
			TotalBudget:  100,
			HistoryRatio: 0.5, // 50 tokens for history
		},
	})

	history := make([]map[string]interface{}, 0, 20)
	for i := 0; i < 20; i++ {
		history = append(history, map[string]interface{}{
			"role":    "user",
			"content": "This is a moderately long message to test truncation behavior.",
		})
	}

	truncated := retriever.TruncateHistory(history)
	if len(truncated) >= len(history) {
		t.Errorf("expected truncation, got %d from %d", len(truncated), len(history))
	}
	if len(truncated) == 0 {
		t.Error("should keep at least some messages")
	}
}

func TestMemoryRetriever_EmptyStore(t *testing.T) {
	retriever := NewMemoryRetriever(MemoryRetrieverOptions{
		Budget: DefaultTokenBudgetConfig(),
	})

	result, err := retriever.Retrieve(nil, "anything", 5)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "" {
		t.Error("expected empty text for empty store")
	}
}

func TestEstimateTextTokens(t *testing.T) {
	short := estimateTextTokens("hello")
	if short <= 0 {
		t.Error("should estimate > 0 for non-empty text")
	}

	long := estimateTextTokens("This is a longer piece of text that should estimate more tokens")
	if long <= short {
		t.Error("longer text should estimate more tokens")
	}

	empty := estimateTextTokens("")
	if empty != 0 {
		t.Error("empty text should estimate 0 tokens")
	}
}
