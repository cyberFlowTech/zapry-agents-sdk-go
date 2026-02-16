package agentsdk

import (
	"log"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Types
// ──────────────────────────────────────────────

// TriggerContext is passed to CheckFn and MessageFn on each poll cycle.
type TriggerContext struct {
	Now       time.Time
	Today     string // "2006-01-02"
	Scheduler *ProactiveScheduler
	State     map[string]interface{}
}

// CheckFn checks whether a trigger should fire.
// It returns a list of user IDs that should receive a message.
// Return nil or empty slice to skip.
type CheckFn func(ctx *TriggerContext) []string

// MessageFn generates the message text for a specific user.
// Return empty string to skip sending.
type MessageFn func(ctx *TriggerContext, userID string) string

// SendFn delivers a text message to a user. Injected by the caller.
type SendFn func(userID string, text string) error

// ──────────────────────────────────────────────
// UserStore interface (optional persistence)
// ──────────────────────────────────────────────

// UserStore manages per-user trigger enable/disable state and send tracking.
// Provide a custom implementation for database persistence.
// If nil, ProactiveScheduler uses an in-memory store.
type UserStore interface {
	IsEnabled(userID, triggerName string) bool
	Enable(userID, triggerName string)
	Disable(userID, triggerName string)
	GetEnabledUsers(triggerName string) []string
	RecordSent(userID, triggerName string, sentAt time.Time)
	AlreadySentToday(userID, triggerName string) bool
}

// ──────────────────────────────────────────────
// InMemoryUserStore (default)
// ──────────────────────────────────────────────

// InMemoryUserStore is a thread-safe, in-memory UserStore.
// Data is lost on restart.
type InMemoryUserStore struct {
	mu       sync.RWMutex
	enabled  map[string]map[string]bool // triggerName -> userID -> true
	sentDate map[string]string          // "userID|triggerName" -> "2006-01-02"
}

// NewInMemoryUserStore creates a new in-memory user store.
func NewInMemoryUserStore() *InMemoryUserStore {
	return &InMemoryUserStore{
		enabled:  make(map[string]map[string]bool),
		sentDate: make(map[string]string),
	}
}

func (s *InMemoryUserStore) IsEnabled(userID, triggerName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users, ok := s.enabled[triggerName]
	if !ok {
		return false
	}
	return users[userID]
}

func (s *InMemoryUserStore) Enable(userID, triggerName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.enabled[triggerName] == nil {
		s.enabled[triggerName] = make(map[string]bool)
	}
	s.enabled[triggerName][userID] = true
}

func (s *InMemoryUserStore) Disable(userID, triggerName string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if users, ok := s.enabled[triggerName]; ok {
		delete(users, userID)
	}
}

func (s *InMemoryUserStore) GetEnabledUsers(triggerName string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	users, ok := s.enabled[triggerName]
	if !ok {
		return nil
	}
	result := make([]string, 0, len(users))
	for uid := range users {
		result = append(result, uid)
	}
	return result
}

func (s *InMemoryUserStore) RecordSent(userID, triggerName string, sentAt time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := userID + "|" + triggerName
	s.sentDate[key] = sentAt.Format("2006-01-02")
}

func (s *InMemoryUserStore) AlreadySentToday(userID, triggerName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := userID + "|" + triggerName
	return s.sentDate[key] == time.Now().Format("2006-01-02")
}

// ──────────────────────────────────────────────
// Trigger
// ──────────────────────────────────────────────

// Trigger holds the check and message functions for a named trigger.
type Trigger struct {
	Name      string
	CheckFn   CheckFn
	MessageFn MessageFn
}

// ──────────────────────────────────────────────
// ProactiveScheduler
// ──────────────────────────────────────────────

// ProactiveScheduler runs periodic checks and sends proactive messages.
//
// Usage:
//
//	scheduler := agentsdk.NewProactiveScheduler(60*time.Second, sendFn, nil)
//
//	scheduler.AddTrigger("daily_greeting", checkGreeting, greetingMessage)
//
//	scheduler.Start()   // non-blocking, starts a background goroutine
//	defer scheduler.Stop()
//
//	scheduler.EnableUser("user_001")
type ProactiveScheduler struct {
	Interval  time.Duration
	SendFn    SendFn
	UserStore UserStore
	State     map[string]interface{}

	mu       sync.RWMutex
	triggers map[string]*Trigger
	stopCh   chan struct{}
	running  bool
}

// NewProactiveScheduler creates a new scheduler.
//
// Parameters:
//   - interval: polling interval (e.g. 60*time.Second)
//   - sendFn: callback to deliver messages to users
//   - userStore: optional UserStore for persistence (nil = in-memory)
func NewProactiveScheduler(interval time.Duration, sendFn SendFn, userStore UserStore) *ProactiveScheduler {
	if userStore == nil {
		userStore = NewInMemoryUserStore()
	}
	return &ProactiveScheduler{
		Interval:  interval,
		SendFn:    sendFn,
		UserStore: userStore,
		State:     make(map[string]interface{}),
		triggers:  make(map[string]*Trigger),
		stopCh:    make(chan struct{}),
	}
}

// AddTrigger registers a named trigger with check and message functions.
func (s *ProactiveScheduler) AddTrigger(name string, checkFn CheckFn, messageFn MessageFn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.triggers[name] = &Trigger{
		Name:      name,
		CheckFn:   checkFn,
		MessageFn: messageFn,
	}
	log.Printf("[ProactiveScheduler] Trigger registered: %s", name)
}

// RemoveTrigger removes a trigger by name.
func (s *ProactiveScheduler) RemoveTrigger(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.triggers, name)
}

// Start launches the background poll loop. Non-blocking.
func (s *ProactiveScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopCh = make(chan struct{})
	s.mu.Unlock()

	go s.pollLoop()
	log.Printf("[ProactiveScheduler] Started (interval=%s)", s.Interval)
}

// Stop halts the background poll loop.
func (s *ProactiveScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
	log.Println("[ProactiveScheduler] Stopped")
}

// ─── User-level toggle ───

// EnableUser enables all registered triggers for the given user.
func (s *ProactiveScheduler) EnableUser(userID string, triggerNames ...string) {
	s.mu.RLock()
	names := triggerNames
	if len(names) == 0 {
		names = make([]string, 0, len(s.triggers))
		for n := range s.triggers {
			names = append(names, n)
		}
	}
	s.mu.RUnlock()

	for _, name := range names {
		s.UserStore.Enable(userID, name)
	}
}

// DisableUser disables all registered triggers for the given user.
func (s *ProactiveScheduler) DisableUser(userID string, triggerNames ...string) {
	s.mu.RLock()
	names := triggerNames
	if len(names) == 0 {
		names = make([]string, 0, len(s.triggers))
		for n := range s.triggers {
			names = append(names, n)
		}
	}
	s.mu.RUnlock()

	for _, name := range names {
		s.UserStore.Disable(userID, name)
	}
}

// IsUserEnabled checks if the user has any (or a specific) trigger enabled.
func (s *ProactiveScheduler) IsUserEnabled(userID string, triggerName string) bool {
	if triggerName != "" {
		return s.UserStore.IsEnabled(userID, triggerName)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for name := range s.triggers {
		if s.UserStore.IsEnabled(userID, name) {
			return true
		}
	}
	return false
}

// ─── Core loop ───

func (s *ProactiveScheduler) pollLoop() {
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.runAllTriggers()
		}
	}
}

func (s *ProactiveScheduler) runAllTriggers() {
	now := time.Now()
	ctx := &TriggerContext{
		Now:       now,
		Today:     now.Format("2006-01-02"),
		Scheduler: s,
		State:     s.State,
	}

	s.mu.RLock()
	triggers := make([]*Trigger, 0, len(s.triggers))
	for _, t := range s.triggers {
		triggers = append(triggers, t)
	}
	s.mu.RUnlock()

	for _, trigger := range triggers {
		s.runTrigger(ctx, trigger)
	}
}

func (s *ProactiveScheduler) runTrigger(ctx *TriggerContext, trigger *Trigger) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ProactiveScheduler] Trigger %q panic: %v", trigger.Name, r)
		}
	}()

	userIDs := trigger.CheckFn(ctx)
	if len(userIDs) == 0 {
		return
	}

	if trigger.MessageFn == nil {
		log.Printf("[ProactiveScheduler] Trigger %q returned users but has no MessageFn", trigger.Name)
		return
	}

	for _, userID := range userIDs {
		if s.UserStore.AlreadySentToday(userID, trigger.Name) {
			continue
		}

		text := trigger.MessageFn(ctx, userID)
		if text == "" {
			continue
		}

		if s.SendFn != nil {
			if err := s.SendFn(userID, text); err != nil {
				log.Printf("[ProactiveScheduler] Send failed | trigger=%s user=%s error=%v",
					trigger.Name, userID, err)
				continue
			}
		} else {
			log.Printf("[ProactiveScheduler] SendFn not set, skipping send to %s", userID)
			continue
		}

		s.UserStore.RecordSent(userID, trigger.Name, ctx.Now)
		log.Printf("[ProactiveScheduler] Sent | trigger=%s user=%s", trigger.Name, userID)
	}
}
