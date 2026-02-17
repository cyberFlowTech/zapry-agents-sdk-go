package agentsdk

import "strings"

// ──────────────────────────────────────────────
// PromptFragments — structured output from NaturalConversation.Enhance
// ──────────────────────────────────────────────

// PromptFragments collects multiple prompt additions and structured metadata
// produced by the natural conversation enhancement pipeline.
type PromptFragments struct {
	// SystemAdditions holds strategy prompt segments to inject as extra_context.
	// Joined with "\n\n" when calling Text().
	SystemAdditions []string

	// KV holds structured metadata with sdk.* namespaced keys.
	// Business logic can read these without parsing prompt text.
	KV map[string]interface{}

	// Warnings records each decision/action for debugging (not injected into LLM).
	// Examples: "style.truncated:exceeded_300", "tone.anxious:followup_boost"
	Warnings []string
}

// NewPromptFragments creates an empty PromptFragments.
func NewPromptFragments() *PromptFragments {
	return &PromptFragments{
		KV: make(map[string]interface{}),
	}
}

// Text returns all SystemAdditions joined as a single string for LLM injection.
func (f *PromptFragments) Text() string {
	if len(f.SystemAdditions) == 0 {
		return ""
	}
	return strings.Join(f.SystemAdditions, "\n\n")
}

// AddSystem appends a strategy prompt segment.
func (f *PromptFragments) AddSystem(text string) {
	if text != "" {
		f.SystemAdditions = append(f.SystemAdditions, text)
	}
}

// AddWarning records a debug message.
func (f *PromptFragments) AddWarning(msg string) {
	f.Warnings = append(f.Warnings, msg)
}

// SetKV sets a namespaced key-value pair.
func (f *PromptFragments) SetKV(key string, value interface{}) {
	f.KV[key] = value
}
