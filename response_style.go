package agentsdk

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// ──────────────────────────────────────────────
// Response Style Controller — local post-processing (no LLM cost)
// ──────────────────────────────────────────────

// StyleConfig controls response style enforcement.
type StyleConfig struct {
	MaxLength        int      // rune limit, default 300, 0=disabled
	MinPreserve      int      // minimum runes to keep even if over MaxLength, default 40
	PreferredLength  int      // hint in prompt, default 150
	ForbiddenPhrases []string // phrases to remove
	EndStyle         string   // "no_question" = convert trailing ? to .
	EnableRetry      bool     // advanced: retry via LLM if violations found (default false)
}

// DefaultStyleConfig returns production-ready defaults.
func DefaultStyleConfig() StyleConfig {
	return StyleConfig{
		MaxLength:       300,
		MinPreserve:     40,
		PreferredLength: 150,
		ForbiddenPhrases: []string{
			"作为一个AI", "作为AI助手", "作为一个人工智能",
			"我是一个AI", "我是AI助手",
			"有什么我可以帮你的", "还有什么需要帮忙的",
			"请问还有什么", "很高兴为你服务",
			"希望对你有帮助", "如果你有任何问题",
		},
		EndStyle:    "no_question",
		EnableRetry: false,
	}
}

// Natural ending sentences for truncation (random selection).
var naturalEndings = []string{
	"先说到这儿。",
	"大概就是这样。",
	"就先聊这些吧。",
	"回头再细说。",
}

var (
	styleRngOnce sync.Once
	styleRng     *rand.Rand
)

func getStyleRng() *rand.Rand {
	styleRngOnce.Do(func() {
		styleRng = rand.New(rand.NewSource(time.Now().UnixNano()))
	})
	return styleRng
}

// ResponseStyleController enforces response style rules via local post-processing.
type ResponseStyleController struct {
	config StyleConfig
}

// NewResponseStyleController creates a style controller.
func NewResponseStyleController(config ...StyleConfig) *ResponseStyleController {
	cfg := DefaultStyleConfig()
	if len(config) > 0 {
		cfg = config[0]
	}
	return &ResponseStyleController{config: cfg}
}

// BuildStylePrompt generates a style constraint prompt segment for LLM injection.
func (c *ResponseStyleController) BuildStylePrompt() string {
	var parts []string
	if c.config.PreferredLength > 0 {
		parts = append(parts, fmt.Sprintf("回复请控制在%d字以内，简洁为主。", c.config.PreferredLength))
	}
	if c.config.EndStyle == "no_question" {
		parts = append(parts, "回复结尾不要以问句结束。")
	}
	if len(parts) == 0 {
		return ""
	}
	return "[回复风格] " + strings.Join(parts, " ")
}

// PostProcess applies local corrections to LLM output (no LLM call).
// Returns: corrected text, whether changes were made, list of violations.
func (c *ResponseStyleController) PostProcess(output string) (string, bool, []string) {
	result := output
	changed := false
	var violations []string

	// 1. Remove forbidden phrases
	for _, phrase := range c.config.ForbiddenPhrases {
		if strings.Contains(result, phrase) {
			result = strings.ReplaceAll(result, phrase, "")
			violations = append(violations, fmt.Sprintf("style.forbidden_removed:%s", phrase))
			changed = true
		}
	}

	// Clean up double spaces/newlines from removals
	if changed {
		result = cleanupWhitespace(result)
	}

	// 2. Truncate if too long (with MinPreserve protection)
	runeCount := utf8.RuneCountInString(result)
	if c.config.MaxLength > 0 && runeCount > c.config.MaxLength && runeCount > c.config.MinPreserve {
		truncated := truncateNatural(result, c.config.MaxLength)
		if truncated != result {
			result = truncated
			violations = append(violations, fmt.Sprintf("style.truncated:exceeded_%d", c.config.MaxLength))
			changed = true
		}
	}

	// 3. Fix trailing question mark
	if c.config.EndStyle == "no_question" {
		trimmed := strings.TrimSpace(result)
		if strings.HasSuffix(trimmed, "？") {
			result = strings.TrimSpace(trimmed[:len(trimmed)-len("？")]) + "。"
			violations = append(violations, "style.end_question_fixed")
			changed = true
		} else if strings.HasSuffix(trimmed, "?") {
			result = strings.TrimSpace(trimmed[:len(trimmed)-1]) + "."
			violations = append(violations, "style.end_question_fixed")
			changed = true
		}
	}

	return strings.TrimSpace(result), changed, violations
}

// BuildRetryPrompt generates a retry prompt when PostProcess violations are found.
// Advanced feature — only use when StyleConfig.EnableRetry is true.
func (c *ResponseStyleController) BuildRetryPrompt(output string, violations []string) string {
	var hints []string
	for _, v := range violations {
		if strings.HasPrefix(v, "style.truncated") {
			hints = append(hints, fmt.Sprintf("请将回复控制在%d字以内", c.config.MaxLength))
		}
		if strings.HasPrefix(v, "style.forbidden") {
			hints = append(hints, "不要使用套话，直接回答")
		}
		if v == "style.end_question_fixed" {
			hints = append(hints, "回复结尾不要以问号结束")
		}
	}
	if len(hints) == 0 {
		return ""
	}
	return "[重新生成] 上一次回复不满足风格要求：" + strings.Join(hints, "；") + "。请重新回复。"
}

// truncateNatural truncates text to maxRunes at the nearest sentence boundary,
// then appends a random natural ending sentence.
func truncateNatural(text string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}

	// Find nearest sentence boundary before maxRunes
	sentenceEnds := []rune{'。', '！', '？', '.', '!', '?', '\n'}
	bestCut := maxRunes
	for i := maxRunes - 1; i >= maxRunes/2; i-- {
		for _, sep := range sentenceEnds {
			if runes[i] == sep {
				bestCut = i + 1
				goto found
			}
		}
	}
found:
	truncated := strings.TrimSpace(string(runes[:bestCut]))

	// Append random natural ending
	rng := getStyleRng()
	ending := naturalEndings[rng.Intn(len(naturalEndings))]
	return truncated + ending
}

func cleanupWhitespace(s string) string {
	// Collapse multiple newlines into two
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	// Collapse multiple spaces into one
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}
