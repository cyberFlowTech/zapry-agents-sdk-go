package persona

import "unicode/utf8"

// TokenEstimator estimates token count from text.
// v1 uses CharsEstimator; v1.5 can plug in a real tokenizer (e.g., tiktoken).
type TokenEstimator interface {
	Estimate(text string) int
}

// CharsEstimator is the v1 default: estimates tokens from rune count.
// zh-CN: ~1.5 chars per token; en: ~4 chars per token.
type CharsEstimator struct {
	CharsPerToken float64
}

// NewCharsEstimator creates a CharsEstimator with the given ratio.
func NewCharsEstimator(charsPerToken float64) *CharsEstimator {
	if charsPerToken <= 0 {
		charsPerToken = 1.5 // zh-CN default
	}
	return &CharsEstimator{CharsPerToken: charsPerToken}
}

// Estimate returns the estimated token count for the given text.
func (e *CharsEstimator) Estimate(text string) int {
	runeCount := utf8.RuneCountInString(text)
	return int(float64(runeCount) / e.CharsPerToken)
}

// RuneCount returns the number of runes in a string.
func RuneCount(s string) int {
	return utf8.RuneCountInString(s)
}
