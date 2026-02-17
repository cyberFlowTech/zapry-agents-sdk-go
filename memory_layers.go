package agentsdk

import (
	"encoding/json"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// WorkingMemory — ephemeral in-memory session context
// ──────────────────────────────────────────────

// WorkingMemory stores temporary data for the current session.
// Not persisted — lost when the object is garbage collected.
type WorkingMemory struct {
	mu   sync.RWMutex
	data map[string]interface{}
}

// NewWorkingMemory creates a new empty working memory.
func NewWorkingMemory() *WorkingMemory {
	return &WorkingMemory{data: make(map[string]interface{})}
}

func (w *WorkingMemory) Get(key string) interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.data[key]
}

func (w *WorkingMemory) Set(key string, value interface{}) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.data[key] = value
}

func (w *WorkingMemory) Delete(key string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.data, key)
}

func (w *WorkingMemory) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.data = make(map[string]interface{})
}

func (w *WorkingMemory) ToMap() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cp := make(map[string]interface{}, len(w.data))
	for k, v := range w.data {
		cp[k] = v
	}
	return cp
}

func (w *WorkingMemory) Len() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.data)
}

// ─── Typed atomic operations (thread-safe) ───

// GetInt returns the int value for key, or 0 if not set or wrong type.
func (w *WorkingMemory) GetInt(key string) int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if v, ok := w.data[key].(int); ok {
		return v
	}
	return 0
}

// SetInt stores an int value.
func (w *WorkingMemory) SetInt(key string, val int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.data[key] = val
}

// Incr atomically increments an int value by 1 and returns the new value.
// If the key does not exist or is not an int, it starts from 0.
func (w *WorkingMemory) Incr(key string) int {
	w.mu.Lock()
	defer w.mu.Unlock()
	v, _ := w.data[key].(int)
	v++
	w.data[key] = v
	return v
}

// GetString returns the string value for key, or "" if not set or wrong type.
func (w *WorkingMemory) GetString(key string) string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if v, ok := w.data[key].(string); ok {
		return v
	}
	return ""
}

// SetString stores a string value.
func (w *WorkingMemory) SetString(key string, val string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.data[key] = val
}

// ──────────────────────────────────────────────
// ShortTermMemory — conversation history
// ──────────────────────────────────────────────

// ShortTermMemory manages recent conversation with automatic trimming.
type ShortTermMemory struct {
	store       MemoryStore
	namespace   string
	maxMessages int
}

const stmKey = "short_term"

// NewShortTermMemory creates a short-term memory manager.
func NewShortTermMemory(store MemoryStore, namespace string, maxMessages int) *ShortTermMemory {
	if maxMessages <= 0 {
		maxMessages = 40
	}
	return &ShortTermMemory{store: store, namespace: namespace, maxMessages: maxMessages}
}

// AddMessage appends a message and auto-trims.
func (s *ShortTermMemory) AddMessage(role, content string) error {
	msg := NewMemoryMessage(role, content)
	data, _ := json.Marshal(msg)
	if err := s.store.Append(s.namespace, stmKey, string(data)); err != nil {
		return err
	}
	return s.store.TrimList(s.namespace, stmKey, s.maxMessages)
}

// GetHistory returns recent messages (oldest first).
func (s *ShortTermMemory) GetHistory(limit int) ([]MemoryMessage, error) {
	if limit <= 0 {
		limit = s.maxMessages
	}
	raw, err := s.store.GetList(s.namespace, stmKey, limit, 0)
	if err != nil {
		return nil, err
	}
	msgs := make([]MemoryMessage, 0, len(raw))
	for _, r := range raw {
		var m MemoryMessage
		if json.Unmarshal([]byte(r), &m) == nil {
			msgs = append(msgs, m)
		}
	}
	return msgs, nil
}

// GetHistoryMaps returns history as []map for LLM messages.
func (s *ShortTermMemory) GetHistoryMaps(limit int) ([]map[string]string, error) {
	msgs, err := s.GetHistory(limit)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]string, len(msgs))
	for i, m := range msgs {
		result[i] = map[string]string{"role": m.Role, "content": m.Content}
	}
	return result, nil
}

// Clear removes all messages.
func (s *ShortTermMemory) Clear() error {
	return s.store.ClearList(s.namespace, stmKey)
}

// Count returns the number of stored messages.
func (s *ShortTermMemory) Count() (int, error) {
	return s.store.ListLength(s.namespace, stmKey)
}

// ──────────────────────────────────────────────
// LongTermMemory — persistent user profile
// ──────────────────────────────────────────────

// LongTermMemory manages persistent user profile/preferences with caching.
type LongTermMemory struct {
	store     MemoryStore
	namespace string
	cacheTTL  time.Duration
	cache     map[string]interface{}
	cacheTS   time.Time
	mu        sync.Mutex
}

const ltmKey = "long_term"

// NewLongTermMemory creates a long-term memory manager.
func NewLongTermMemory(store MemoryStore, namespace string, cacheTTL time.Duration) *LongTermMemory {
	return &LongTermMemory{
		store:     store,
		namespace: namespace,
		cacheTTL:  cacheTTL,
	}
}

// Get loads the long-term memory (using cache if fresh).
func (l *LongTermMemory) Get() (map[string]interface{}, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cache != nil && l.cacheTTL > 0 && time.Since(l.cacheTS) < l.cacheTTL {
		return copyMap(l.cache), nil
	}

	raw, err := l.store.Get(l.namespace, ltmKey)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if raw != "" {
		if json.Unmarshal([]byte(raw), &data) != nil {
			data = defaultSchema()
		}
	} else {
		data = defaultSchema()
		if meta, ok := data["meta"].(map[string]interface{}); ok {
			meta["created_at"] = time.Now().Format(time.RFC3339)
		}
	}

	l.cache = data
	l.cacheTS = time.Now()
	return copyMap(data), nil
}

// Save overwrites the entire long-term memory.
func (l *LongTermMemory) Save(data map[string]interface{}) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if meta, ok := data["meta"].(map[string]interface{}); ok {
		meta["updated_at"] = time.Now().Format(time.RFC3339)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if err := l.store.Set(l.namespace, ltmKey, string(raw)); err != nil {
		return err
	}
	l.cache = copyMap(data)
	l.cacheTS = time.Now()
	return nil
}

// Update deep-merges updates into existing memory.
func (l *LongTermMemory) Update(updates map[string]interface{}) (map[string]interface{}, error) {
	current, err := l.Get()
	if err != nil {
		return nil, err
	}
	merged := DeepMerge(current, updates)
	if meta, ok := merged["meta"].(map[string]interface{}); ok {
		count, _ := meta["conversation_count"].(float64)
		meta["conversation_count"] = count + 1
		meta["updated_at"] = time.Now().Format(time.RFC3339)
	}
	if err := l.Save(merged); err != nil {
		return nil, err
	}
	return merged, nil
}

// DeleteMem removes all long-term memory.
func (l *LongTermMemory) DeleteMem() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cache = nil
	l.cacheTS = time.Time{}
	return l.store.Delete(l.namespace, ltmKey)
}

// InvalidateCache forces next Get() to reload.
func (l *LongTermMemory) InvalidateCache() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.cache = nil
}

// GetCached returns the cached data (may be nil).
func (l *LongTermMemory) GetCached() map[string]interface{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.cache == nil {
		return nil
	}
	return copyMap(l.cache)
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

func defaultSchema() map[string]interface{} {
	return map[string]interface{}{
		"basic_info":   map[string]interface{}{},
		"personality":  map[string]interface{}{},
		"life_context": map[string]interface{}{},
		"interests":    []interface{}{},
		"summary":      "",
		"preferences":  map[string]interface{}{},
		"meta": map[string]interface{}{
			"conversation_count": float64(0),
			"created_at":         "",
			"updated_at":         "",
		},
	}
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	data, _ := json.Marshal(m)
	var cp map[string]interface{}
	json.Unmarshal(data, &cp)
	return cp
}

// DeepMerge recursively merges override into base.
func DeepMerge(base, override map[string]interface{}) map[string]interface{} {
	result := copyMap(base)
	for k, v := range override {
		if v == nil {
			continue
		}
		existing, exists := result[k]
		if !exists {
			result[k] = v
			continue
		}
		baseMap, baseIsMap := existing.(map[string]interface{})
		overMap, overIsMap := v.(map[string]interface{})
		if baseIsMap && overIsMap {
			result[k] = DeepMerge(baseMap, overMap)
			continue
		}
		baseSlice, baseIsSlice := existing.([]interface{})
		overSlice, overIsSlice := v.([]interface{})
		if baseIsSlice && overIsSlice {
			seen := make(map[string]bool)
			for _, item := range baseSlice {
				seen[stringify(item)] = true
			}
			for _, item := range overSlice {
				if !seen[stringify(item)] {
					baseSlice = append(baseSlice, item)
				}
			}
			result[k] = baseSlice
			continue
		}
		result[k] = v
	}
	return result
}

func stringify(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}
