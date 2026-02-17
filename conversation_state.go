package agentsdk

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// ──────────────────────────────────────────────
// Conversation State Tracker — automatic dialogue metadata
// ──────────────────────────────────────────────

// ConversationState holds automatically computed dialogue metadata.
// All fields are derived from MemorySession + current message, zero LLM cost.
type ConversationState struct {
	TurnIndex           int           `json:"turn_index"`
	IsFollowUp          bool          `json:"is_followup"`
	IsFirstConversation bool          `json:"is_first_conversation"`
	SessionDuration     time.Duration `json:"session_duration"`
	DaysSinceLast       int           `json:"days_since_last"` // -1 = first conversation
	TotalSessions       int           `json:"total_sessions"`
	TimeOfDay           string        `json:"time_of_day"`     // morning/afternoon/evening/late_night
	UserMsgLength       string        `json:"user_msg_length"` // short/medium/long
	LocalTime           string        `json:"local_time"`      // RFC3339 with timezone
}

const (
	stateMetaKey          = "sdk.conversation_meta"
	stateTurnKey          = "sdk.session.turn_index"
	stateLastMsgAtKey     = "sdk.session.last_msg_at"
	stateSessionStartKey  = "sdk.session.start_at"
)

// conversationMeta is persisted in MemoryStore.
type conversationMeta struct {
	TotalSessions int    `json:"total_sessions"`
	LastAt        string `json:"last_at"` // RFC3339
}

// ConversationStateTracker computes ConversationState automatically.
type ConversationStateTracker struct {
	followUpWindow time.Duration
	timezone       *time.Location
}

// NewConversationStateTracker creates a tracker. Timezone defaults to "Asia/Shanghai".
func NewConversationStateTracker(timezone ...string) *ConversationStateTracker {
	tz := "Asia/Shanghai"
	if len(timezone) > 0 && timezone[0] != "" {
		tz = timezone[0]
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	return &ConversationStateTracker{
		followUpWindow: 60 * time.Second,
		timezone:       loc,
	}
}

// Track computes the ConversationState for the current user message.
// It reads/writes metadata from session's WorkingMemory and MemoryStore.
func (t *ConversationStateTracker) Track(session *MemorySession, userInput string, now time.Time) *ConversationState {
	wm := session.Working
	localNow := now.In(t.timezone)

	// Turn index (session-level, in WorkingMemory)
	turnIndex := wm.Incr(stateTurnKey)

	// Session start time
	if wm.GetString(stateSessionStartKey) == "" {
		wm.SetString(stateSessionStartKey, now.Format(time.RFC3339))
	}

	// Session duration
	var sessionDuration time.Duration
	if startStr := wm.GetString(stateSessionStartKey); startStr != "" {
		if startTime, err := time.Parse(time.RFC3339, startStr); err == nil {
			sessionDuration = now.Sub(startTime)
		}
	}

	// Is follow-up (based on last message time in this session)
	isFollowUp := false
	if lastStr := wm.GetString(stateLastMsgAtKey); lastStr != "" {
		if lastTime, err := time.Parse(time.RFC3339, lastStr); err == nil {
			if now.Sub(lastTime) <= t.followUpWindow {
				isFollowUp = true
			}
		}
	}
	wm.SetString(stateLastMsgAtKey, now.Format(time.RFC3339))

	// Load persisted meta
	meta := t.loadMeta(session)

	// Days since last
	daysSinceLast := -1
	if meta.LastAt != "" {
		if lastAt, err := time.Parse(time.RFC3339, meta.LastAt); err == nil {
			days := int(now.Sub(lastAt).Hours() / 24)
			if days < 0 {
				days = 0
			}
			daysSinceLast = days
		}
	}

	// Time of day
	timeOfDay := classifyTimeOfDay(localNow.Hour())

	// User message length
	msgLen := utf8.RuneCountInString(userInput)
	userMsgLength := classifyMsgLength(msgLen)

	return &ConversationState{
		TurnIndex:           turnIndex,
		IsFollowUp:          isFollowUp,
		IsFirstConversation: daysSinceLast == -1,
		SessionDuration:     sessionDuration,
		DaysSinceLast:       daysSinceLast,
		TotalSessions:       meta.TotalSessions,
		TimeOfDay:           timeOfDay,
		UserMsgLength:       userMsgLength,
		LocalTime:           localNow.Format(time.RFC3339),
	}
}

// TouchSession increments TotalSessions and updates LastAt.
// Call this once per session (e.g. after first message).
func (t *ConversationStateTracker) TouchSession(session *MemorySession, now time.Time) {
	meta := t.loadMeta(session)
	meta.TotalSessions++
	meta.LastAt = now.Format(time.RFC3339)
	t.saveMeta(session, &meta)
}

func (t *ConversationStateTracker) loadMeta(session *MemorySession) conversationMeta {
	var meta conversationMeta
	raw, err := session.store.Get(session.Namespace, stateMetaKey)
	if err == nil && raw != "" {
		json.Unmarshal([]byte(raw), &meta)
	}
	return meta
}

func (t *ConversationStateTracker) saveMeta(session *MemorySession, meta *conversationMeta) {
	data, _ := json.Marshal(meta)
	session.store.Set(session.Namespace, stateMetaKey, string(data))
}

// FormatForPrompt returns a strategy prompt segment for LLM injection.
func (s *ConversationState) FormatForPrompt() string {
	var lines []string
	lines = append(lines, "[对话状态]")

	if s.IsFirstConversation {
		lines = append(lines, "- 这是你们的第一次对话")
	} else {
		lines = append(lines, fmt.Sprintf("- 这是你们的第%d次对话，本次会话第%d轮", s.TotalSessions, s.TurnIndex))
		if s.DaysSinceLast > 0 {
			lines = append(lines, fmt.Sprintf("- 距离上次对话已过去%d天", s.DaysSinceLast))
		}
	}

	if s.IsFollowUp {
		lines = append(lines, "- 用户正在追问，请直接回应，不要寒暄")
	}

	switch s.TimeOfDay {
	case "late_night":
		lines = append(lines, "- 当前时间：深夜")
	case "morning":
		lines = append(lines, "- 当前时间：上午")
	case "evening":
		lines = append(lines, "- 当前时间：晚上")
	}

	switch s.UserMsgLength {
	case "short":
		lines = append(lines, "- 用户消息较短，回复也保持简短")
	case "long":
		lines = append(lines, "- 用户消息较长，可以给出详细回复")
	}

	return strings.Join(lines, "\n")
}

// ToKV returns structured key-value pairs with sdk.* namespace.
func (s *ConversationState) ToKV() map[string]interface{} {
	return map[string]interface{}{
		"sdk.conversation.days_since_last": s.DaysSinceLast,
		"sdk.conversation.total_sessions":  s.TotalSessions,
		"sdk.conversation.is_first":        s.IsFirstConversation,
		"sdk.session.turn_index":           s.TurnIndex,
		"sdk.session.duration_sec":         int(s.SessionDuration.Seconds()),
		"sdk.user.is_followup":             s.IsFollowUp,
		"sdk.user.msg_length":              s.UserMsgLength,
		"sdk.runtime.time_of_day":          s.TimeOfDay,
		"sdk.runtime.local_time":           s.LocalTime,
	}
}

func classifyTimeOfDay(hour int) string {
	switch {
	case hour >= 6 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 18:
		return "afternoon"
	case hour >= 18 && hour < 23:
		return "evening"
	default:
		return "late_night"
	}
}

func classifyMsgLength(runeCount int) string {
	switch {
	case runeCount < 20:
		return "short"
	case runeCount <= 120:
		return "medium"
	default:
		return "long"
	}
}
