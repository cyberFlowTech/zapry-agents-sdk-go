package persona

import (
	"strings"
	"unicode/utf8"

)

// TrimToBudget trims the tick injection and constraints text to fit within maxChars.
// Priority (lowest trimmed first):
// 1. Today event in injection (can remove)
// 2. Activity details (can simplify)
// 3. Constraints text (never trim)
func TrimToBudget(injection string, constraintsText string, maxChars int) (string, string, int) {
	total := utf8.RuneCountInString(injection) + utf8.RuneCountInString(constraintsText)
	if total <= maxChars {
		return injection, constraintsText, total
	}

	// Step 1: Remove today event
	if idx := strings.Index(injection, "\n【今天的小事】"); idx >= 0 {
		injection = injection[:idx]
		total = utf8.RuneCountInString(injection) + utf8.RuneCountInString(constraintsText)
		if total <= maxChars {
			return injection, constraintsText, total
		}
	}

	// Step 2: Simplify injection to just the state line
	if idx := strings.Index(injection, "。"); idx >= 0 {
		// Keep only up to first period
		runes := []rune(injection)
		for i, r := range runes {
			if r == '。' {
				injection = string(runes[:i+1])
				break
			}
		}
		total = utf8.RuneCountInString(injection) + utf8.RuneCountInString(constraintsText)
		if total <= maxChars {
			return injection, constraintsText, total
		}
	}

	// Step 3: Hard truncate injection (preserve constraints)
	constraintsLen := utf8.RuneCountInString(constraintsText)
	injectionBudget := maxChars - constraintsLen
	if injectionBudget < 0 {
		injectionBudget = 0
	}
	runes := []rune(injection)
	if len(runes) > injectionBudget {
		injection = string(runes[:injectionBudget])
	}

	total = utf8.RuneCountInString(injection) + utf8.RuneCountInString(constraintsText)
	return injection, constraintsText, total
}

// AdjustParams creates model parameter overrides based on mood.
func AdjustParams(base ModelParams, mood *MoodState) map[string]any {
	overrides := map[string]any{}

	// Low energy → slightly lower temperature (more stable, warmer)
	if mood.Value < 40 {
		overrides["temperature"] = base.Temperature - 0.05
	} else if mood.Value > 70 {
		overrides["temperature"] = base.Temperature + 0.03
	}

	return overrides
}
