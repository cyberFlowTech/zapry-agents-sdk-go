package agentsdk

import "fmt"

// ──────────────────────────────────────────────
// Conversation Opener Generator — strategy hints for natural openings
// ──────────────────────────────────────────────

// OpenerStrategy holds the generated opening strategy for the current turn.
type OpenerStrategy struct {
	Situation string // first_meeting/long_absence/followup/late_night/normal
	Hint      string // strategy hint for LLM (not fixed text)
}

// OpenerConfig controls opener generation behavior.
type OpenerConfig struct {
	MaxMentionsPerSession int // max opener injections per session, default 1
	LongAbsenceDays       int // days threshold for "long_absence", default 3
}

// DefaultOpenerConfig returns production defaults.
func DefaultOpenerConfig() OpenerConfig {
	return OpenerConfig{
		MaxMentionsPerSession: 1,
		LongAbsenceDays:       3,
	}
}

// OpenerGenerator creates opening strategy hints based on conversation state.
type OpenerGenerator struct {
	config OpenerConfig
}

// NewOpenerGenerator creates an opener generator.
func NewOpenerGenerator(config ...OpenerConfig) *OpenerGenerator {
	cfg := DefaultOpenerConfig()
	if len(config) > 0 {
		cfg = config[0]
	}
	return &OpenerGenerator{config: cfg}
}

// Generate produces an OpenerStrategy based on ConversationState.
// sessionOpenerCount is how many times opener has been injected in this session.
// If frequency limit is reached, returns Situation="normal" with empty Hint.
func (g *OpenerGenerator) Generate(state *ConversationState, sessionOpenerCount int) *OpenerStrategy {
	// Frequency limit
	if sessionOpenerCount >= g.config.MaxMentionsPerSession {
		return &OpenerStrategy{Situation: "normal", Hint: ""}
	}

	// Priority order: followup > first_meeting > long_absence > late_night > normal
	if state.IsFollowUp {
		return &OpenerStrategy{
			Situation: "followup",
			Hint:      "用户在追问，不要寒暄，直接回应上一个问题。",
		}
	}

	if state.IsFirstConversation {
		return &OpenerStrategy{
			Situation: "first_meeting",
			Hint:      "这是你们第一次对话，自然地打个招呼，不要问「有什么可以帮你的」。",
		}
	}

	if state.DaysSinceLast >= g.config.LongAbsenceDays {
		return &OpenerStrategy{
			Situation: "long_absence",
			Hint:      fmt.Sprintf("距离上次对话已经%d天了，可以自然地表达「好久没聊了」的意思，但不要太正式。", state.DaysSinceLast),
		}
	}

	if state.TimeOfDay == "late_night" {
		return &OpenerStrategy{
			Situation: "late_night",
			Hint:      "现在是深夜，语气可以更轻松随意，如果用户聊到很晚可以温柔提醒。",
		}
	}

	return &OpenerStrategy{Situation: "normal", Hint: ""}
}

// FormatForPrompt returns the hint as a prompt segment (empty if no hint).
func (s *OpenerStrategy) FormatForPrompt() string {
	if s.Hint == "" {
		return ""
	}
	return "[开场策略] " + s.Hint
}
