package agentsdk

import (
	"testing"
	"time"
)

func TestTypedMemoryStore_AddAndGet(t *testing.T) {
	store := NewInMemoryMemoryStore()
	ts := NewTypedMemoryStore(store, "test:user1")

	mem := TypedMemory{
		ID:      "fact-1",
		Type:    MemoryTypeSemantic,
		Content: "User is 25 years old",
		Score:   0.8,
	}
	if err := ts.Add(mem); err != nil {
		t.Fatal(err)
	}

	got, err := ts.Get(MemoryTypeSemantic, "fact-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.Content != "User is 25 years old" {
		t.Errorf("unexpected content: %s", got.Content)
	}
	if got.AccessCnt != 1 {
		t.Errorf("access count should be 1, got %d", got.AccessCnt)
	}
}

func TestTypedMemoryStore_ListByType(t *testing.T) {
	store := NewInMemoryMemoryStore()
	ts := NewTypedMemoryStore(store, "test:user1")

	ts.Add(TypedMemory{ID: "s1", Type: MemoryTypeSemantic, Content: "fact 1", Score: 0.8})
	ts.Add(TypedMemory{ID: "s2", Type: MemoryTypeSemantic, Content: "fact 2", Score: 0.6})
	ts.Add(TypedMemory{ID: "e1", Type: MemoryTypeEpisodic, Content: "experience 1", Score: 0.7})

	semantics, err := ts.ListByType(MemoryTypeSemantic)
	if err != nil {
		t.Fatal(err)
	}
	if len(semantics) != 2 {
		t.Errorf("expected 2 semantic, got %d", len(semantics))
	}

	episodics, _ := ts.ListByType(MemoryTypeEpisodic)
	if len(episodics) != 1 {
		t.Errorf("expected 1 episodic, got %d", len(episodics))
	}
}

func TestTypedMemoryStore_Update(t *testing.T) {
	store := NewInMemoryMemoryStore()
	ts := NewTypedMemoryStore(store, "test:user1")

	ts.Add(TypedMemory{ID: "f1", Type: MemoryTypeSemantic, Content: "age 25", Score: 0.8})

	mem, _ := ts.Get(MemoryTypeSemantic, "f1")
	mem.Content = "age 26"
	mem.Score = 0.9
	ts.Update(*mem)

	updated, _ := ts.Get(MemoryTypeSemantic, "f1")
	if updated.Content != "age 26" {
		t.Errorf("expected updated content, got %s", updated.Content)
	}
}

func TestTypedMemoryStore_Remove(t *testing.T) {
	store := NewInMemoryMemoryStore()
	ts := NewTypedMemoryStore(store, "test:user1")

	ts.Add(TypedMemory{ID: "f1", Type: MemoryTypeSemantic, Content: "fact", Score: 0.8})
	ts.Remove(MemoryTypeSemantic, "f1")

	got, _ := ts.Get(MemoryTypeSemantic, "f1")
	if got != nil {
		t.Error("expected nil after removal")
	}
}

func TestTypedMemoryStore_ClearType(t *testing.T) {
	store := NewInMemoryMemoryStore()
	ts := NewTypedMemoryStore(store, "test:user1")

	ts.Add(TypedMemory{ID: "s1", Type: MemoryTypeSemantic, Content: "fact", Score: 0.8})
	ts.Add(TypedMemory{ID: "e1", Type: MemoryTypeEpisodic, Content: "exp", Score: 0.7})

	ts.ClearType(MemoryTypeSemantic)

	semantics, _ := ts.ListByType(MemoryTypeSemantic)
	if len(semantics) != 0 {
		t.Errorf("expected 0 semantic after clear, got %d", len(semantics))
	}

	episodics, _ := ts.ListByType(MemoryTypeEpisodic)
	if len(episodics) != 1 {
		t.Error("episodic should not be affected")
	}
}

func TestTypedMemoryStore_FormatForPrompt(t *testing.T) {
	store := NewInMemoryMemoryStore()
	ts := NewTypedMemoryStore(store, "test:user1")

	ts.Add(TypedMemory{ID: "s1", Type: MemoryTypeSemantic, Content: "User is 25", Score: 0.8})
	ts.Add(TypedMemory{ID: "p1", Type: MemoryTypeProcedural, Content: "Prefer concise replies", Score: 0.9})

	text := ts.FormatForPrompt()
	if text == "" {
		t.Error("expected non-empty prompt text")
	}
}

func TestTypedMemoryStore_ClearAll(t *testing.T) {
	store := NewInMemoryMemoryStore()
	ts := NewTypedMemoryStore(store, "test:user1")

	ts.Add(TypedMemory{ID: "s1", Type: MemoryTypeSemantic, Content: "a", Score: 0.5})
	ts.Add(TypedMemory{ID: "e1", Type: MemoryTypeEpisodic, Content: "b", Score: 0.5})
	ts.Add(TypedMemory{ID: "p1", Type: MemoryTypeProcedural, Content: "c", Score: 0.5})

	ts.ClearAll()

	for _, mt := range []MemoryType{MemoryTypeSemantic, MemoryTypeEpisodic, MemoryTypeProcedural} {
		mems, _ := ts.ListByType(mt)
		if len(mems) != 0 {
			t.Errorf("expected 0 for %s, got %d", mt, len(mems))
		}
	}
}

func TestTypedMemory_Timestamps(t *testing.T) {
	store := NewInMemoryMemoryStore()
	ts := NewTypedMemoryStore(store, "test:user1")

	ts.Add(TypedMemory{ID: "f1", Type: MemoryTypeSemantic, Content: "test", Score: 0.5})

	mem, _ := ts.Get(MemoryTypeSemantic, "f1")
	if mem.CreatedAt.IsZero() {
		t.Error("created_at should be set")
	}
	if mem.UpdatedAt.IsZero() {
		t.Error("updated_at should be set")
	}
	if mem.UpdatedAt.Before(mem.CreatedAt) {
		t.Error("updated_at should be >= created_at")
	}

	_ = time.Now()
}
