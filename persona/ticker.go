package persona

import (
	"time"

)

// LocalTicker generates PersonaTick locally without network calls.
type LocalTicker struct{}

// NewLocalTicker creates a new LocalTicker.
func NewLocalTicker() *LocalTicker {
	return &LocalTicker{}
}

// Tick generates a PersonaTick for the current moment.
func (t *LocalTicker) Tick(config *RuntimeConfig, userID string, now time.Time, recentEventIDs []string) *PersonaTick {
	// 1. Resolve current state from time slots
	state := ResolveState(config, now)

	// 2. Select today's event (with 3-day dedup)
	todayEvent := SelectTodayEvent(config.TodayEventPool, config.PersonaID, now, recentEventIDs)

	// 3. Calculate mood
	mood := CalculateMood(config.MoodModel.BaseMood, state.Energy)
	state.Mood = mood.Label

	// 4. Build style constraints
	styleJSON := BuildStyleConstraints(config.StylePolicy)
	styleText := RenderStyleConstraintsText(styleJSON)

	// 5. Build prompt injection
	injection := BuildPromptInjection(state, &mood, todayEvent, now, config)

	// 6. Model param override based on mood
	paramOverride := AdjustParams(config.ModelParams, &mood)

	// 7. Budget tracking
	budgetUsed := RuneCount(injection) + RuneCount(styleText)

	// 8. Apply budget trimming
	maxTickChars := config.PromptBudget.MaxTickInjectionChars
	if maxTickChars > 0 && budgetUsed > maxTickChars {
		injection, styleText, budgetUsed = TrimToBudget(injection, styleText, maxTickChars)
	}

	return &PersonaTick{
		PersonaID:            config.PersonaID,
		TickTime:             now,
		CurrentState:         *state,
		PromptInjection:      injection,
		TodayEvent:           todayEvent,
		StyleConstraintsJSON: styleJSON,
		StyleConstraintsText: styleText,
		ModelParamOverride:   paramOverride,
		PromptBudgetUsed:     budgetUsed,
	}
}

// BuildStyleConstraints creates structured style constraints from policy.
func BuildStyleConstraints(policy StylePolicy) StyleConstraints {
	return StyleConstraints{
		MustShareFirst:           policy.ShareFirst,
		MaxQuestionsThisTurn:     policy.MaxQuestionsPerTurn,
		AvoidEndWithQuestionMark: true,
		PreferShortReply:         false,
		BlockedPhrases:           []string{"我在等你", "你想聊什么", "有什么我可以帮你的"},
	}
}
