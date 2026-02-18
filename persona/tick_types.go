package persona

import (
	"fmt"
	"time"
)

// PersonaTick is the runtime output generated before each LLM call.
// It contains state injection, style constraints, and parameter overrides.
type PersonaTick struct {
	PersonaID    string       `json:"persona_id"`
	TickTime     time.Time    `json:"tick_time"`
	CurrentState CurrentState `json:"current_state"`

	PromptInjection string `json:"prompt_injection"`        // 50-200 chars natural language state
	TodayEvent      string `json:"today_event,omitempty"`   // daily event from pool

	// Dual-format style constraints
	StyleConstraintsJSON StyleConstraints `json:"style_constraints"`      // structured, for programmatic detection
	StyleConstraintsText string           `json:"style_constraints_text"` // rendered text, for direct prompt injection

	ModelParamOverride map[string]any `json:"model_param_override,omitempty"`

	// Budget tracking
	PromptBudgetUsed int `json:"prompt_budget_used"` // total chars injected by this tick
}

// CurrentState represents the persona's current activity and energy.
type CurrentState struct {
	Activity string `json:"activity"`
	Mood     string `json:"mood"`
	Energy   int    `json:"energy"` // 0-100
}

// StyleConstraints are the structured behavior rules for this turn.
type StyleConstraints struct {
	MustShareFirst          bool     `json:"must_share_first"`
	MaxQuestionsThisTurn    int      `json:"max_questions_this_turn"`
	AvoidEndWithQuestionMark bool    `json:"avoid_end_with_question_mark"`
	PreferShortReply        bool     `json:"prefer_short_reply"`
	BlockedPhrases          []string `json:"blocked_phrases,omitempty"`
}

// RenderStyleConstraintsText renders the style constraints as
// a human-readable text block for prompt injection.
func RenderStyleConstraintsText(sc StyleConstraints) string {
	text := "【行为规则 - 本轮生效】\n"

	if sc.MustShareFirst {
		text += "1. 你必须先分享 1-2 句自己的状态、感受或联想，然后再接用户的话题\n"
	}

	if sc.MaxQuestionsThisTurn == 0 {
		text += "2. 本轮不要提问\n"
	} else {
		text += fmt.Sprintf("2. 本轮最多问 %d 个问题\n", sc.MaxQuestionsThisTurn)
	}

	if sc.AvoidEndWithQuestionMark {
		text += "3. 不要以问号结尾\n"
	}

	text += "4. 不使用「我在等你聊天」「你想聊什么」之类的空话\n"
	text += "5. 用具体细节: 提到具体地点、物品、动作或感受\n"
	text += "6. 如果用户冷场: 主动抛出一个轻量话题(音乐/天气/小事)，不要追问\n"

	return text
}
