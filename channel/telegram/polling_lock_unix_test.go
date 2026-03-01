//go:build !windows

package telegram

import "testing"

func TestPollingInstanceLock_SameTokenConflict(t *testing.T) {
	lock1, err := acquirePollingInstanceLock("test-token-same")
	if err != nil {
		t.Fatalf("first lock should succeed, got error: %v", err)
	}
	defer func() {
		_ = lock1.Release()
	}()

	lock2, err := acquirePollingInstanceLock("test-token-same")
	if err == nil {
		_ = lock2.Release()
		t.Fatal("second lock with same token should fail")
	}
}

func TestPollingInstanceLock_DifferentTokenAllowed(t *testing.T) {
	lock1, err := acquirePollingInstanceLock("test-token-a")
	if err != nil {
		t.Fatalf("lock for token A should succeed, got error: %v", err)
	}
	defer func() {
		_ = lock1.Release()
	}()

	lock2, err := acquirePollingInstanceLock("test-token-b")
	if err != nil {
		t.Fatalf("lock for token B should succeed, got error: %v", err)
	}
	defer func() {
		_ = lock2.Release()
	}()
}
