package agentsdk

import "time"

// Message represents a single conversation message.
type MemoryMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp,omitempty"`
}

// NewMemoryMessage creates a message with the current timestamp.
func NewMemoryMessage(role, content string) MemoryMessage {
	return MemoryMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now().Format(time.RFC3339),
	}
}

// MemoryContext holds a snapshot of all three memory layers.
type MemoryContext struct {
	Working   map[string]interface{}
	ShortTerm []MemoryMessage
	LongTerm  map[string]interface{}
}
