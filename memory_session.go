package agentsdk

import (
	"fmt"
	"log"
	"time"
)

// MemorySession is the high-level convenience API for managing all three memory layers.
//
// Usage:
//
//	session := agentsdk.NewMemorySession("my_agent", "user_123", store)
//	ctx, _ := session.Load()
//	session.AddMessage("user", "Hello!")
//	prompt := session.FormatForPrompt("")
//	session.ExtractIfNeeded()
type MemorySession struct {
	AgentID   string
	UserID    string
	Namespace string

	Working   *WorkingMemory
	ShortTerm *ShortTermMemory
	LongTerm  *LongTermMemory
	Buffer    *ConversationBuffer

	store     MemoryStore
	extractor MemoryExtractorInterface
}

// NewMemorySession creates a session with default settings.
func NewMemorySession(agentID, userID string, store MemoryStore) *MemorySession {
	ns := fmt.Sprintf("%s:%s", agentID, userID)
	return &MemorySession{
		AgentID:   agentID,
		UserID:    userID,
		Namespace: ns,
		Working:   NewWorkingMemory(),
		ShortTerm: NewShortTermMemory(store, ns, 40),
		LongTerm:  NewLongTermMemory(store, ns, 5*time.Minute),
		Buffer:    NewConversationBuffer(store, ns, 5, 24*time.Hour),
		store:     store,
	}
}

// NewMemorySessionWithOptions creates a session with custom settings.
func NewMemorySessionWithOptions(
	agentID, userID string,
	store MemoryStore,
	maxMessages int,
	cacheTTL time.Duration,
	triggerCount int,
	triggerInterval time.Duration,
) *MemorySession {
	ns := fmt.Sprintf("%s:%s", agentID, userID)
	return &MemorySession{
		AgentID:   agentID,
		UserID:    userID,
		Namespace: ns,
		Working:   NewWorkingMemory(),
		ShortTerm: NewShortTermMemory(store, ns, maxMessages),
		LongTerm:  NewLongTermMemory(store, ns, cacheTTL),
		Buffer:    NewConversationBuffer(store, ns, triggerCount, triggerInterval),
		store:     store,
	}
}

// SetExtractor sets the memory extractor for automatic extraction.
func (s *MemorySession) SetExtractor(ext MemoryExtractorInterface) {
	s.extractor = ext
}

// Load loads all memory layers and returns a snapshot.
func (s *MemorySession) Load() (*MemoryContext, error) {
	history, err := s.ShortTerm.GetHistory(0)
	if err != nil {
		return nil, err
	}
	ltData, err := s.LongTerm.Get()
	if err != nil {
		return nil, err
	}
	return &MemoryContext{
		Working:   s.Working.ToMap(),
		ShortTerm: history,
		LongTerm:  ltData,
	}, nil
}

// AddMessage adds to both short-term history and conversation buffer.
func (s *MemorySession) AddMessage(role, content string) error {
	if err := s.ShortTerm.AddMessage(role, content); err != nil {
		return err
	}
	return s.Buffer.Add(role, content)
}

// ExtractIfNeeded checks triggers and extracts memory if needed.
// Returns the extracted delta, or nil.
func (s *MemorySession) ExtractIfNeeded() map[string]interface{} {
	if s.extractor == nil {
		return nil
	}

	should, err := s.Buffer.ShouldExtract()
	if err != nil || !should {
		return nil
	}

	conversations, err := s.Buffer.GetAndClear()
	if err != nil || len(conversations) == 0 {
		return nil
	}

	current, err := s.LongTerm.Get()
	if err != nil {
		return nil
	}

	extracted, err := s.extractor.Extract(conversations, current)
	if err != nil {
		log.Printf("[MemorySession] Extraction failed: %v", err)
		return nil
	}

	if len(extracted) > 0 {
		s.LongTerm.Update(extracted)
		log.Printf("[MemorySession] Memory extracted | ns=%s", s.Namespace)
	}

	return extracted
}

// FormatForPrompt formats current memory for LLM prompt injection.
func (s *MemorySession) FormatForPrompt(template string) string {
	lt := s.LongTerm.GetCached()
	if lt == nil {
		lt = map[string]interface{}{}
	}
	wm := s.Working.ToMap()
	return FormatMemoryForPrompt(lt, wm, template)
}

// UpdateLongTerm updates long-term memory with incremental changes.
func (s *MemorySession) UpdateLongTerm(updates map[string]interface{}) (map[string]interface{}, error) {
	return s.LongTerm.Update(updates)
}

// ClearHistory clears short-term history only.
func (s *MemorySession) ClearHistory() error {
	return s.ShortTerm.Clear()
}

// ClearBuffer clears the conversation buffer only.
func (s *MemorySession) ClearBuffer() error {
	return s.Buffer.Clear()
}

// ClearAll clears all memory layers.
func (s *MemorySession) ClearAll() error {
	s.Working.Clear()
	if err := s.ShortTerm.Clear(); err != nil {
		return err
	}
	if err := s.LongTerm.DeleteMem(); err != nil {
		return err
	}
	return s.Buffer.Clear()
}
