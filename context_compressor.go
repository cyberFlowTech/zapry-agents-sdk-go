package agentsdk

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ──────────────────────────────────────────────
// Context Compressor — intelligent conversation history compression
// ──────────────────────────────────────────────

// SummarizeFn calls the LLM to generate a conversation summary.
type SummarizeFn func(messages []map[string]interface{}) (string, error)

// EstimateTokensFn estimates the token count of conversation history.
// Default implementation: runeCount / 2.7 with code block weighting.
// Users can provide a real tokenizer.
type EstimateTokensFn func(history []map[string]interface{}) int

// CompressorConfig controls context compression behavior.
type CompressorConfig struct {
	WindowSize       int              // recent messages to keep intact, default 6
	TokenThreshold   int              // compress only when estimated tokens exceed this, default 6000
	SummaryVersion   string           // cache version tag, change to invalidate, default "v1"
	EstimateTokensFn EstimateTokensFn // pluggable token estimator, nil = default
}

// DefaultCompressorConfig returns production defaults.
func DefaultCompressorConfig() CompressorConfig {
	return CompressorConfig{
		WindowSize:     6,
		TokenThreshold: 6000,
		SummaryVersion: "v1",
	}
}

// ContextCompressor compresses conversation history using LLM summarization.
type ContextCompressor struct {
	config      CompressorConfig
	summarizeFn SummarizeFn
}

// NewContextCompressor creates a compressor. SummarizeFn is required.
func NewContextCompressor(fn SummarizeFn, config ...CompressorConfig) *ContextCompressor {
	cfg := DefaultCompressorConfig()
	if len(config) > 0 {
		cfg = config[0]
	}
	return &ContextCompressor{
		config:      cfg,
		summarizeFn: fn,
	}
}

// Compress compresses conversation history if estimated tokens exceed threshold.
// Returns: [summary_system_msg] + recent WindowSize messages.
// Summary is cached in WorkingMemory; cache is invalidated when SummaryVersion changes.
// If summarization fails, returns original history unchanged.
func (c *ContextCompressor) Compress(
	history []map[string]interface{},
	working *WorkingMemory,
) ([]map[string]interface{}, error) {
	if len(history) == 0 {
		return history, nil
	}

	// Estimate tokens
	tokens := c.estimateTokens(history)
	if tokens < c.config.TokenThreshold {
		return history, nil
	}

	// Check cache
	cacheKey := fmt.Sprintf("sdk.context_summary:%s", c.config.SummaryVersion)
	if cached := working.GetString(cacheKey); cached != "" {
		return c.buildCompressedHistory(cached, history), nil
	}

	// Split: old messages to summarize + recent to keep
	splitIdx := len(history) - c.config.WindowSize
	if splitIdx <= 0 {
		return history, nil
	}

	oldMessages := history[:splitIdx]

	// Summarize
	summary, err := c.summarizeFn(oldMessages)
	if err != nil {
		// Fallback: return original
		return history, nil
	}

	// Cache
	working.SetString(cacheKey, summary)

	return c.buildCompressedHistory(summary, history), nil
}

func (c *ContextCompressor) buildCompressedHistory(summary string, history []map[string]interface{}) []map[string]interface{} {
	// Tag the summary with version for debugging
	taggedSummary := fmt.Sprintf("[sdk.summary:%s] %s", c.config.SummaryVersion, summary)

	recentStart := len(history) - c.config.WindowSize
	if recentStart < 0 {
		recentStart = 0
	}

	result := make([]map[string]interface{}, 0, 1+c.config.WindowSize)
	result = append(result, map[string]interface{}{
		"role":    "system",
		"content": taggedSummary,
	})
	result = append(result, history[recentStart:]...)
	return result
}

func (c *ContextCompressor) estimateTokens(history []map[string]interface{}) int {
	if c.config.EstimateTokensFn != nil {
		return c.config.EstimateTokensFn(history)
	}
	return defaultEstimateTokens(history)
}

// defaultEstimateTokens estimates tokens as runeCount / 2.7.
// Code blocks get 1.5x weight.
func defaultEstimateTokens(history []map[string]interface{}) int {
	totalRunes := 0
	for _, msg := range history {
		content, _ := msg["content"].(string)
		runes := utf8.RuneCountInString(content)
		// Code block detection: simple heuristic
		if strings.Contains(content, "```") {
			runes = int(float64(runes) * 1.5)
		}
		totalRunes += runes
	}
	return int(float64(totalRunes) / 2.7)
}
