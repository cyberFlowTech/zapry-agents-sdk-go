package agentsdk

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func newTestRedisStore(t *testing.T) *RedisMemoryStore {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	store, err := NewRedisMemoryStore(RedisMemoryStoreOptions{
		Addr:             mr.Addr(),
		KeyPrefix:        "test:memory",
		OperationTimeout: 1 * time.Second,
	})
	if err != nil {
		t.Fatalf("create redis memory store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestRedisMemoryStore_KVOperations(t *testing.T) {
	store := newTestRedisStore(t)

	if err := store.Set("ns1", "k1", "v1"); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	got, err := store.Get("ns1", "k1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if got != "v1" {
		t.Fatalf("expected v1, got %q", got)
	}

	if err := store.Delete("ns1", "k1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	got, err = store.Get("ns1", "k1")
	if err != nil {
		t.Fatalf("get after delete failed: %v", err)
	}
	if got != "" {
		t.Fatalf("expected empty string after delete, got %q", got)
	}
}

func TestRedisMemoryStore_ListOperations(t *testing.T) {
	store := newTestRedisStore(t)

	for _, item := range []string{"a", "b", "c", "d"} {
		if err := store.Append("ns2", "list", item); err != nil {
			t.Fatalf("append failed: %v", err)
		}
	}

	items, err := store.GetList("ns2", "list", 2, 1)
	if err != nil {
		t.Fatalf("get list failed: %v", err)
	}
	if len(items) != 2 || items[0] != "b" || items[1] != "c" {
		t.Fatalf("unexpected list slice: %v", items)
	}

	if err := store.TrimList("ns2", "list", 2); err != nil {
		t.Fatalf("trim list failed: %v", err)
	}
	items, err = store.GetList("ns2", "list", 0, 0)
	if err != nil {
		t.Fatalf("get list after trim failed: %v", err)
	}
	if len(items) != 2 || items[0] != "c" || items[1] != "d" {
		t.Fatalf("unexpected list after trim: %v", items)
	}

	length, err := store.ListLength("ns2", "list")
	if err != nil {
		t.Fatalf("list length failed: %v", err)
	}
	if length != 2 {
		t.Fatalf("expected length=2, got %d", length)
	}

	if err := store.ClearList("ns2", "list"); err != nil {
		t.Fatalf("clear list failed: %v", err)
	}
	length, err = store.ListLength("ns2", "list")
	if err != nil {
		t.Fatalf("list length after clear failed: %v", err)
	}
	if length != 0 {
		t.Fatalf("expected length=0 after clear, got %d", length)
	}
}

func TestRedisMemoryStore_ListKeys(t *testing.T) {
	store := newTestRedisStore(t)

	_ = store.Set("ns3", "profile", "ok")
	_ = store.Append("ns3", "history", "h1")
	_ = store.Append("ns3", "history", "h2")
	_ = store.Set("ns4", "other", "x") // different namespace

	keys, err := store.ListKeys("ns3")
	if err != nil {
		t.Fatalf("list keys failed: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys in ns3, got %d (%v)", len(keys), keys)
	}
	if !(containsString(keys, "history") && containsString(keys, "profile")) {
		t.Fatalf("unexpected keys: %v", keys)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
