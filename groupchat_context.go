package agentsdk

import (
	"fmt"
	"time"
)

// ──────────────────────────────────────────────
// GroupChat — Shared Context (message history visible to all agents)
// ──────────────────────────────────────────────

// GroupMessage is a message in a group chat.
type GroupMessage struct {
	SenderID        string   // sender ID (human or agent)
	SenderName      string   // display name
	Content         string   // message text
	IsFromAgent     bool     // true if sent by an Agent
	MentionedAgents []string // @mentioned agent IDs
	Timestamp       time.Time
}

// GroupReply is an Agent's response to a group message.
type GroupReply struct {
	AgentID   string // responding agent ID
	AgentName string // agent display name
	Content   string // reply text
	Reason    string // why this agent replied: mention/skill/followup/talkativeness
}

// SharedContext manages the shared message history for a group chat.
// All agents in the group see the same history.
type SharedContext struct {
	history    []map[string]interface{}
	maxHistory int
}

// NewSharedContext creates a shared context with optional max history limit.
func NewSharedContext(maxHistory ...int) *SharedContext {
	max := 50
	if len(maxHistory) > 0 && maxHistory[0] > 0 {
		max = maxHistory[0]
	}
	return &SharedContext{maxHistory: max}
}

// Append adds a user/human message to the shared history.
func (sc *SharedContext) Append(msg GroupMessage) {
	entry := map[string]interface{}{
		"role":    "user",
		"content": fmt.Sprintf("%s: %s", msg.SenderName, msg.Content),
		"name":    msg.SenderName,
	}
	if msg.IsFromAgent {
		entry["role"] = "assistant"
	}
	sc.history = append(sc.history, entry)
	sc.trim()
}

// AppendReply adds an Agent reply to the shared history.
func (sc *SharedContext) AppendReply(reply GroupReply) {
	sc.history = append(sc.history, map[string]interface{}{
		"role":    "assistant",
		"content": fmt.Sprintf("%s: %s", reply.AgentName, reply.Content),
		"name":    reply.AgentName,
	})
	sc.trim()
}

// GetHistory returns the shared history as LLM messages format.
func (sc *SharedContext) GetHistory() []map[string]interface{} {
	result := make([]map[string]interface{}, len(sc.history))
	copy(result, sc.history)
	return result
}

// Len returns the number of messages in history.
func (sc *SharedContext) Len() int {
	return len(sc.history)
}

func (sc *SharedContext) trim() {
	if sc.maxHistory > 0 && len(sc.history) > sc.maxHistory {
		sc.history = sc.history[len(sc.history)-sc.maxHistory:]
	}
}
