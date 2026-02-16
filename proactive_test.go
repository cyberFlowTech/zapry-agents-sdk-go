package agentsdk

import (
	"sync"
	"testing"
	"time"
)

// ══════════════════════════════════════════════
// InMemoryUserStore tests
// ══════════════════════════════════════════════

func TestInMemoryUserStore_EnableDisable(t *testing.T) {
	store := NewInMemoryUserStore()

	if store.IsEnabled("u1", "daily") {
		t.Fatal("should not be enabled by default")
	}

	store.Enable("u1", "daily")
	if !store.IsEnabled("u1", "daily") {
		t.Fatal("should be enabled after Enable()")
	}

	store.Disable("u1", "daily")
	if store.IsEnabled("u1", "daily") {
		t.Fatal("should be disabled after Disable()")
	}
}

func TestInMemoryUserStore_GetEnabledUsers(t *testing.T) {
	store := NewInMemoryUserStore()
	store.Enable("u1", "daily")
	store.Enable("u2", "daily")
	store.Enable("u3", "birthday")

	users := store.GetEnabledUsers("daily")
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	found := map[string]bool{}
	for _, u := range users {
		found[u] = true
	}
	if !found["u1"] || !found["u2"] {
		t.Fatalf("expected u1 and u2, got %v", users)
	}
}

func TestInMemoryUserStore_AlreadySentToday(t *testing.T) {
	store := NewInMemoryUserStore()

	if store.AlreadySentToday("u1", "daily") {
		t.Fatal("should not be sent by default")
	}

	store.RecordSent("u1", "daily", time.Now())
	if !store.AlreadySentToday("u1", "daily") {
		t.Fatal("should be marked as sent today")
	}
}

func TestInMemoryUserStore_EmptyTrigger(t *testing.T) {
	store := NewInMemoryUserStore()
	users := store.GetEnabledUsers("nonexistent")
	if users != nil {
		t.Fatalf("expected nil, got %v", users)
	}
}

// ══════════════════════════════════════════════
// ProactiveScheduler tests
// ══════════════════════════════════════════════

func TestProactiveScheduler_StartStop(t *testing.T) {
	s := NewProactiveScheduler(100*time.Millisecond, nil, nil)
	s.Start()
	if !s.running {
		t.Fatal("scheduler should be running")
	}
	s.Stop()
	if s.running {
		t.Fatal("scheduler should be stopped")
	}
}

func TestProactiveScheduler_StartIdempotent(t *testing.T) {
	s := NewProactiveScheduler(100*time.Millisecond, nil, nil)
	s.Start()
	s.Start() // should not panic or create double goroutines
	s.Stop()
}

func TestProactiveScheduler_AddRemoveTrigger(t *testing.T) {
	s := NewProactiveScheduler(time.Second, nil, nil)
	s.AddTrigger("test", func(ctx *TriggerContext) []string { return nil }, nil)

	s.mu.RLock()
	_, ok := s.triggers["test"]
	s.mu.RUnlock()
	if !ok {
		t.Fatal("trigger should be registered")
	}

	s.RemoveTrigger("test")
	s.mu.RLock()
	_, ok = s.triggers["test"]
	s.mu.RUnlock()
	if ok {
		t.Fatal("trigger should be removed")
	}
}

func TestProactiveScheduler_EnableDisableUser(t *testing.T) {
	s := NewProactiveScheduler(time.Second, nil, nil)
	s.AddTrigger("t1", func(ctx *TriggerContext) []string { return nil }, nil)
	s.AddTrigger("t2", func(ctx *TriggerContext) []string { return nil }, nil)

	s.EnableUser("u1")
	if !s.IsUserEnabled("u1", "") {
		t.Fatal("user should be enabled for any trigger")
	}
	if !s.IsUserEnabled("u1", "t1") {
		t.Fatal("user should be enabled for t1")
	}
	if !s.IsUserEnabled("u1", "t2") {
		t.Fatal("user should be enabled for t2")
	}

	s.DisableUser("u1", "t1")
	if s.IsUserEnabled("u1", "t1") {
		t.Fatal("user should be disabled for t1")
	}
	if !s.IsUserEnabled("u1", "t2") {
		t.Fatal("user should still be enabled for t2")
	}

	s.DisableUser("u1")
	if s.IsUserEnabled("u1", "") {
		t.Fatal("user should be disabled for all")
	}
}

func TestProactiveScheduler_TriggerExecution(t *testing.T) {
	var mu sync.Mutex
	var sent []struct {
		userID string
		text   string
	}

	sendFn := func(userID, text string) error {
		mu.Lock()
		sent = append(sent, struct {
			userID string
			text   string
		}{userID, text})
		mu.Unlock()
		return nil
	}

	s := NewProactiveScheduler(time.Second, sendFn, nil)

	s.AddTrigger("greet", func(ctx *TriggerContext) []string {
		return []string{"u1", "u2"}
	}, func(ctx *TriggerContext, userID string) string {
		return "Hello " + userID
	})

	// Manually run all triggers
	s.runAllTriggers()

	mu.Lock()
	defer mu.Unlock()
	if len(sent) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(sent))
	}
}

func TestProactiveScheduler_DedupToday(t *testing.T) {
	var mu sync.Mutex
	count := 0

	sendFn := func(userID, text string) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	}

	s := NewProactiveScheduler(time.Second, sendFn, nil)

	s.AddTrigger("daily", func(ctx *TriggerContext) []string {
		return []string{"u1"}
	}, func(ctx *TriggerContext, userID string) string {
		return "Hi"
	})

	s.runAllTriggers()
	s.runAllTriggers() // second run should be deduped

	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Fatalf("expected 1 send (dedup), got %d", count)
	}
}

func TestProactiveScheduler_EmptyMessageSkip(t *testing.T) {
	count := 0
	sendFn := func(userID, text string) error {
		count++
		return nil
	}

	s := NewProactiveScheduler(time.Second, sendFn, nil)

	s.AddTrigger("skip", func(ctx *TriggerContext) []string {
		return []string{"u1"}
	}, func(ctx *TriggerContext, userID string) string {
		return "" // empty → skip
	})

	s.runAllTriggers()
	if count != 0 {
		t.Fatalf("expected 0 sends (empty message), got %d", count)
	}
}

func TestProactiveScheduler_NoSendFn(t *testing.T) {
	// Should not panic when SendFn is nil
	s := NewProactiveScheduler(time.Second, nil, nil)

	s.AddTrigger("test", func(ctx *TriggerContext) []string {
		return []string{"u1"}
	}, func(ctx *TriggerContext, userID string) string {
		return "Hello"
	})

	s.runAllTriggers() // should not panic
}

func TestProactiveScheduler_StateShared(t *testing.T) {
	s := NewProactiveScheduler(time.Second, nil, nil)

	s.AddTrigger("counter", func(ctx *TriggerContext) []string {
		val, _ := ctx.State["count"].(int)
		ctx.State["count"] = val + 1
		return nil
	}, nil)

	s.runAllTriggers()
	s.runAllTriggers()

	if s.State["count"] != 2 {
		t.Fatalf("expected state count=2, got %v", s.State["count"])
	}
}

func TestProactiveScheduler_PollLoopRuns(t *testing.T) {
	var mu sync.Mutex
	callCount := 0

	s := NewProactiveScheduler(50*time.Millisecond, nil, nil)
	s.AddTrigger("tick", func(ctx *TriggerContext) []string {
		mu.Lock()
		callCount++
		mu.Unlock()
		return nil
	}, nil)

	s.Start()
	time.Sleep(180 * time.Millisecond) // should fire 2-3 times
	s.Stop()

	mu.Lock()
	defer mu.Unlock()
	if callCount < 2 {
		t.Fatalf("expected at least 2 poll cycles, got %d", callCount)
	}
}
