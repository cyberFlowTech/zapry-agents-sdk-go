package agentsdk

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNormalizeAutoConversationOptions_SessionCleanupDefaults(t *testing.T) {
	opts := normalizeAutoConversationOptions(AutoConversationOptions{})
	if opts.SessionTTL <= 0 {
		t.Fatal("expected default session ttl to be enabled")
	}
	if opts.SessionCleanupInterval <= 0 {
		t.Fatal("expected default cleanup interval to be enabled")
	}
}

func TestDefaultAutoConversationOptions_EmptyReplyByLocale(t *testing.T) {
	t.Setenv("LANG", "en_US.UTF-8")
	en := DefaultAutoConversationOptions()
	if strings.Contains(en.EmptyReply, "我在") {
		t.Fatalf("expected english fallback reply, got: %s", en.EmptyReply)
	}

	t.Setenv("LANG", "zh_CN.UTF-8")
	zh := DefaultAutoConversationOptions()
	if !strings.Contains(zh.EmptyReply, "我在") {
		t.Fatalf("expected chinese fallback reply, got: %s", zh.EmptyReply)
	}
}

func TestNormalizeAutoConversationOptions_SessionCleanupDisabled(t *testing.T) {
	opts := normalizeAutoConversationOptions(AutoConversationOptions{DisableSessionCleanup: true})
	if opts.SessionTTL != 0 {
		t.Fatalf("expected session ttl = 0, got %s", opts.SessionTTL)
	}
	if opts.SessionCleanupInterval != 0 {
		t.Fatalf("expected session cleanup interval = 0, got %s", opts.SessionCleanupInterval)
	}
}

func TestAutoConversationRuntime_CleanupExpiredSessions(t *testing.T) {
	rt := &AutoConversationRuntime{
		agentKey: "agent",
		store:    NewInMemoryMemoryStore(),
		options: AutoConversationOptions{
			SessionTTL: 50 * time.Millisecond,
		},
	}
	session := rt.sessionFor("u1")
	if session == nil {
		t.Fatal("expected non-nil session")
	}

	v, ok := rt.sessions.Load("u1")
	if !ok {
		t.Fatal("expected session to exist")
	}
	ref, ok := v.(*autoSessionRef)
	if !ok {
		t.Fatalf("expected *autoSessionRef, got %T", v)
	}
	ref.lastAccess.Store(time.Now().Add(-time.Second).UnixNano())

	rt.cleanupExpiredSessions(time.Now())

	if _, ok := rt.sessions.Load("u1"); ok {
		t.Fatal("expected expired session to be cleaned")
	}
}

func TestAutoConversationRuntime_Shutdown_WaitsInflightRuns(t *testing.T) {
	rt := &AutoConversationRuntime{
		cleanupDoneCh: make(chan struct{}),
	}
	close(rt.cleanupDoneCh)
	rt.activeRuns.Add(1)

	go func() {
		time.Sleep(20 * time.Millisecond)
		rt.activeRuns.Done()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := rt.Shutdown(ctx); err != nil {
		t.Fatalf("unexpected shutdown error: %v", err)
	}
	if !rt.closing.Load() {
		t.Fatal("runtime should be marked as closing")
	}
}

func TestAutoConversationRuntime_ShutdownTimeout(t *testing.T) {
	rt := &AutoConversationRuntime{
		cleanupDoneCh: make(chan struct{}),
	}
	close(rt.cleanupDoneCh)
	rt.activeRuns.Add(1)
	defer rt.activeRuns.Done()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := rt.Shutdown(ctx)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}
