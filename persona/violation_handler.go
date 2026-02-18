package persona

import (
	"time"
)

// ViolationHandler manages retry logic for constraint violations.
type ViolationHandler struct {
	MaxRetries int // Fixed at 1
}

// NewViolationHandler creates a handler with MaxRetries=1.
func NewViolationHandler() *ViolationHandler {
	return &ViolationHandler{MaxRetries: 1}
}

// BuildRetryMessages creates the messages for a retry attempt.
// Uses append-only strategy: keeps original assistant output, adds system instruction.
// Returns nil if no hard violations found.
func (h *ViolationHandler) BuildRetryMessages(
	originalMessages []map[string]interface{},
	assistantOutput string,
	violations []ViolationResult,
) []map[string]interface{} {
	hardViolations := FilterHard(violations)
	if len(hardViolations) == 0 {
		return nil
	}

	// Append original assistant output (don't delete)
	msgs := make([]map[string]interface{}, len(originalMessages))
	copy(msgs, originalMessages)

	msgs = append(msgs, map[string]interface{}{
		"role":    "assistant",
		"content": assistantOutput,
	})

	// Append retry instruction (< 100 chars)
	retryPrompt := "上一条回复违反了对话规则，请重写。要求："
	for _, v := range hardViolations {
		switch v.Type {
		case ViolationEndsWithQuestion:
			retryPrompt += "不要以问号结尾。"
		case ViolationExcessiveQuestions:
			retryPrompt += "最多问1个问题。"
		}
	}
	retryPrompt += "不要引用或提及规则本身。"

	msgs = append(msgs, map[string]interface{}{
		"role":    "system",
		"content": retryPrompt,
	})

	return msgs
}

// ViolationLog is used for observability.
type ViolationLog struct {
	PersonaID      string        `json:"persona_id"`
	ViolationType  ViolationType `json:"violation_type"`
	Severity       string        `json:"severity"`
	RetryAttempted bool          `json:"retry_attempted"`
	RetrySuccess   bool          `json:"retry_success"`
	Timestamp      time.Time     `json:"timestamp"`
}
