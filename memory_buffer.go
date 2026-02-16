package agentsdk

import (
	"encoding/json"
	"log"
	"time"
)

const (
	bufListKey = "buffer"
	bufMetaKey = "buffer_meta"
)

// ConversationBuffer manages a conversation buffer for memory extraction.
type ConversationBuffer struct {
	store           MemoryStore
	namespace       string
	triggerCount    int
	triggerInterval time.Duration
}

// NewConversationBuffer creates a buffer with configurable trigger conditions.
func NewConversationBuffer(store MemoryStore, namespace string, triggerCount int, triggerInterval time.Duration) *ConversationBuffer {
	if triggerCount <= 0 {
		triggerCount = 5
	}
	if triggerInterval <= 0 {
		triggerInterval = 24 * time.Hour
	}
	return &ConversationBuffer{
		store:           store,
		namespace:       namespace,
		triggerCount:    triggerCount,
		triggerInterval: triggerInterval,
	}
}

// Add appends a message to the buffer.
func (b *ConversationBuffer) Add(role, content string) error {
	entry := map[string]string{
		"role":      role,
		"content":   content,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	data, _ := json.Marshal(entry)
	return b.store.Append(b.namespace, bufListKey, string(data))
}

// ShouldExtract checks if extraction should be triggered.
func (b *ConversationBuffer) ShouldExtract() (bool, error) {
	length, err := b.store.ListLength(b.namespace, bufListKey)
	if err != nil {
		return false, err
	}
	if length == 0 {
		return false, nil
	}
	if length >= b.triggerCount {
		return true, nil
	}

	raw, err := b.store.Get(b.namespace, bufMetaKey)
	if err != nil {
		return false, err
	}
	if raw == "" {
		return true, nil // no prior extraction
	}

	var meta map[string]interface{}
	if json.Unmarshal([]byte(raw), &meta) != nil {
		return true, nil
	}
	lastTS, _ := meta["last_extraction_ts"].(float64)
	if time.Since(time.Unix(int64(lastTS), 0)) >= b.triggerInterval {
		return true, nil
	}
	return false, nil
}

// GetAndClear atomically retrieves all messages and clears the buffer.
func (b *ConversationBuffer) GetAndClear() ([]map[string]string, error) {
	raw, err := b.store.GetList(b.namespace, bufListKey, 0, 0)
	if err != nil {
		return nil, err
	}
	if err := b.store.ClearList(b.namespace, bufListKey); err != nil {
		log.Printf("[ConversationBuffer] ClearList error: %v", err)
	}

	meta, _ := json.Marshal(map[string]interface{}{
		"last_extraction_ts": float64(time.Now().Unix()),
		"last_extraction_at": time.Now().Format(time.RFC3339),
	})
	b.store.Set(b.namespace, bufMetaKey, string(meta))

	messages := make([]map[string]string, 0, len(raw))
	for _, r := range raw {
		var m map[string]string
		if json.Unmarshal([]byte(r), &m) == nil {
			messages = append(messages, m)
		}
	}
	return messages, nil
}

// Count returns the current buffer size.
func (b *ConversationBuffer) Count() (int, error) {
	return b.store.ListLength(b.namespace, bufListKey)
}

// Clear empties the buffer without recording extraction.
func (b *ConversationBuffer) Clear() error {
	return b.store.ClearList(b.namespace, bufListKey)
}
