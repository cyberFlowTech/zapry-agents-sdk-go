package agentsdk

import (
	"context"
	"sort"
	"unicode/utf8"
)

// ──────────────────────────────────────────────
// Memory Retriever — query-aware retrieval + token budget
// ──────────────────────────────────────────────

// TokenBudgetConfig controls how much of the prompt token budget
// is allocated to each component.
type TokenBudgetConfig struct {
	TotalBudget  int     // total prompt token budget (e.g. 8000)
	SystemRatio  float64 // system prompt share, default 0.3
	MemoryRatio  float64 // memory share, default 0.2
	HistoryRatio float64 // conversation history share, default 0.5
}

// DefaultTokenBudgetConfig returns production defaults.
func DefaultTokenBudgetConfig() TokenBudgetConfig {
	return TokenBudgetConfig{
		TotalBudget:  8000,
		SystemRatio:  0.3,
		MemoryRatio:  0.2,
		HistoryRatio: 0.5,
	}
}

// BudgetFor returns the token budget for a specific component.
func (c TokenBudgetConfig) BudgetFor(ratio float64) int {
	return int(float64(c.TotalBudget) * ratio)
}

// MemoryBudget returns the token budget allocated to memory.
func (c TokenBudgetConfig) MemoryBudget() int { return c.BudgetFor(c.MemoryRatio) }

// HistoryBudget returns the token budget allocated to history.
func (c TokenBudgetConfig) HistoryBudget() int { return c.BudgetFor(c.HistoryRatio) }

// SystemBudget returns the token budget allocated to the system prompt.
func (c TokenBudgetConfig) SystemBudget() int { return c.BudgetFor(c.SystemRatio) }

// MemoryRetriever performs query-aware memory retrieval with token budget enforcement.
// It combines structured memory (LongTermMemory) with vector search (SemanticMemoryStore)
// and truncates results to fit within the allocated token budget.
type MemoryRetriever struct {
	Semantic   *SemanticMemoryStore
	Structured *LongTermMemory
	Typed      *TypedMemoryStore
	Budget     TokenBudgetConfig
}

// NewMemoryRetriever creates a retriever with the given stores and budget.
func NewMemoryRetriever(opts MemoryRetrieverOptions) *MemoryRetriever {
	budget := opts.Budget
	if budget.TotalBudget <= 0 {
		budget = DefaultTokenBudgetConfig()
	}
	return &MemoryRetriever{
		Semantic:   opts.Semantic,
		Structured: opts.Structured,
		Typed:      opts.Typed,
		Budget:     budget,
	}
}

// MemoryRetrieverOptions groups optional dependencies for MemoryRetriever.
type MemoryRetrieverOptions struct {
	Semantic   *SemanticMemoryStore
	Structured *LongTermMemory
	Typed      *TypedMemoryStore
	Budget     TokenBudgetConfig
}

// RetrievedMemory holds the final assembled memory text ready for prompt injection.
type RetrievedMemory struct {
	Text       string // formatted text for prompt injection
	TokensUsed int    // estimated tokens consumed
	HitCount   int    // number of semantic hits included
}

// Retrieve performs query-aware memory retrieval:
//  1. If SemanticMemoryStore is available, vector-search for top-K relevant memories
//  2. Otherwise, fall back to structured LongTermMemory
//  3. Optionally include TypedMemory entries
//  4. Truncate to fit within the memory token budget
func (r *MemoryRetriever) Retrieve(ctx context.Context, query string, topK int) (*RetrievedMemory, error) {
	budget := r.Budget.MemoryBudget()
	if budget <= 0 {
		budget = 1600
	}
	if topK <= 0 {
		topK = 10
	}

	var sections []scoredSection

	// Vector search (highest priority)
	if r.Semantic != nil && query != "" {
		hits, err := r.Semantic.SearchRelevant(ctx, query, topK, nil)
		if err == nil && len(hits) > 0 {
			for _, h := range hits {
				sections = append(sections, scoredSection{
					text:  h.Content,
					score: float64(h.Score),
				})
			}
		}
	}

	// Typed memories
	if r.Typed != nil {
		text := r.Typed.FormatForPrompt()
		if text != "" {
			sections = append(sections, scoredSection{
				text:  text,
				score: 0.5,
			})
		}
	}

	// Structured fallback (if no vector search or as supplement)
	if r.Structured != nil {
		cached := r.Structured.GetCached()
		if cached == nil {
			cached, _ = r.Structured.Get()
		}
		if cached != nil {
			text := formatLongTerm(cached)
			if text != "" {
				sections = append(sections, scoredSection{
					text:  text,
					score: 0.3,
				})
			}
		}
	}

	if len(sections) == 0 {
		return &RetrievedMemory{}, nil
	}

	// Sort by score descending
	sort.Slice(sections, func(i, j int) bool {
		return sections[i].score > sections[j].score
	})

	// Truncate to budget
	var result []string
	tokensUsed := 0
	hitCount := 0

	for _, s := range sections {
		est := estimateTextTokens(s.text)
		if tokensUsed+est > budget && len(result) > 0 {
			break
		}
		result = append(result, s.text)
		tokensUsed += est
		hitCount++
	}

	finalText := joinLines(result)
	return &RetrievedMemory{
		Text:       finalText,
		TokensUsed: tokensUsed,
		HitCount:   hitCount,
	}, nil
}

// TruncateHistory trims conversation history to fit within the history budget.
func (r *MemoryRetriever) TruncateHistory(history []map[string]interface{}) []map[string]interface{} {
	budget := r.Budget.HistoryBudget()
	if budget <= 0 || len(history) == 0 {
		return history
	}

	// Keep most recent messages that fit within budget, walking backwards
	total := 0
	startIdx := len(history)
	for i := len(history) - 1; i >= 0; i-- {
		content, _ := history[i]["content"].(string)
		est := estimateTextTokens(content)
		if total+est > budget && startIdx < len(history) {
			break
		}
		total += est
		startIdx = i
	}

	return history[startIdx:]
}

type scoredSection struct {
	text  string
	score float64
}

func estimateTextTokens(text string) int {
	runes := utf8.RuneCountInString(text)
	return int(float64(runes) / 2.7)
}
