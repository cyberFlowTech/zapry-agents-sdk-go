package agentsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ──────────────────────────────────────────────
// Typed Memory — Semantic / Episodic / Procedural
// ──────────────────────────────────────────────

// MemoryType distinguishes the semantic category of a memory entry.
type MemoryType string

const (
	MemoryTypeSemantic   MemoryType = "semantic"   // facts, knowledge, user profile
	MemoryTypeEpisodic   MemoryType = "episodic"   // past experiences, notable interactions
	MemoryTypeProcedural MemoryType = "procedural" // behavior rules, user preferences
)

// TypedMemory represents a single memory entry with semantic classification,
// importance scoring, and access tracking.
type TypedMemory struct {
	ID        string            `json:"id"`
	Type      MemoryType        `json:"type"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Score     float64           `json:"score"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	AccessCnt int               `json:"access_cnt"`
}

// TypedMemoryStore manages typed memories backed by a MemoryStore.
// Each memory is serialized as JSON in the KV store under a type-prefixed key.
type TypedMemoryStore struct {
	store     MemoryStore
	namespace string
}

// NewTypedMemoryStore creates a typed memory manager.
func NewTypedMemoryStore(store MemoryStore, namespace string) *TypedMemoryStore {
	return &TypedMemoryStore{store: store, namespace: namespace}
}

func (t *TypedMemoryStore) memKey(memType MemoryType, id string) string {
	return fmt.Sprintf("typed:%s:%s", memType, id)
}

func (t *TypedMemoryStore) indexKey(memType MemoryType) string {
	return fmt.Sprintf("typed_index:%s", memType)
}

// Add stores a new typed memory. The ID must be unique within the namespace+type.
func (t *TypedMemoryStore) Add(mem TypedMemory) error {
	if mem.CreatedAt.IsZero() {
		mem.CreatedAt = time.Now()
	}
	mem.UpdatedAt = time.Now()

	data, err := json.Marshal(mem)
	if err != nil {
		return err
	}

	if err := t.store.Set(t.namespace, t.memKey(mem.Type, mem.ID), string(data)); err != nil {
		return err
	}
	return t.store.Append(t.namespace, t.indexKey(mem.Type), mem.ID)
}

// Get retrieves a single typed memory and increments its access count.
func (t *TypedMemoryStore) Get(memType MemoryType, id string) (*TypedMemory, error) {
	raw, err := t.store.Get(t.namespace, t.memKey(memType, id))
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}

	var mem TypedMemory
	if err := json.Unmarshal([]byte(raw), &mem); err != nil {
		return nil, err
	}

	mem.AccessCnt++
	mem.UpdatedAt = time.Now()
	updated, _ := json.Marshal(mem)
	t.store.Set(t.namespace, t.memKey(memType, id), string(updated))

	return &mem, nil
}

// ListByType returns all memories of a given type.
func (t *TypedMemoryStore) ListByType(memType MemoryType) ([]TypedMemory, error) {
	ids, err := t.store.GetList(t.namespace, t.indexKey(memType), 0, 0)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var memories []TypedMemory
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		raw, err := t.store.Get(t.namespace, t.memKey(memType, id))
		if err != nil || raw == "" {
			continue
		}
		var mem TypedMemory
		if json.Unmarshal([]byte(raw), &mem) == nil {
			memories = append(memories, mem)
		}
	}
	return memories, nil
}

// Update modifies an existing typed memory.
func (t *TypedMemoryStore) Update(mem TypedMemory) error {
	mem.UpdatedAt = time.Now()
	data, err := json.Marshal(mem)
	if err != nil {
		return err
	}
	return t.store.Set(t.namespace, t.memKey(mem.Type, mem.ID), string(data))
}

// Remove deletes a typed memory by type and ID.
func (t *TypedMemoryStore) Remove(memType MemoryType, id string) error {
	return t.store.Delete(t.namespace, t.memKey(memType, id))
}

// Count returns the number of memories of a given type.
func (t *TypedMemoryStore) Count(memType MemoryType) (int, error) {
	return t.store.ListLength(t.namespace, t.indexKey(memType))
}

// ClearType removes all memories of a given type.
func (t *TypedMemoryStore) ClearType(memType MemoryType) error {
	mems, err := t.ListByType(memType)
	if err != nil {
		return err
	}
	for _, m := range mems {
		t.store.Delete(t.namespace, t.memKey(memType, m.ID))
	}
	return t.store.ClearList(t.namespace, t.indexKey(memType))
}

// ClearAll removes all typed memories across all types.
func (t *TypedMemoryStore) ClearAll() error {
	for _, mt := range []MemoryType{MemoryTypeSemantic, MemoryTypeEpisodic, MemoryTypeProcedural} {
		if err := t.ClearType(mt); err != nil {
			return err
		}
	}
	return nil
}

// FormatForPrompt produces a prompt-friendly text representation of all typed memories.
func (t *TypedMemoryStore) FormatForPrompt() string {
	var sections []string

	for _, mt := range []MemoryType{MemoryTypeSemantic, MemoryTypeEpisodic, MemoryTypeProcedural} {
		mems, err := t.ListByType(mt)
		if err != nil || len(mems) == 0 {
			continue
		}
		label := memoryTypeLabel(mt)
		items := make([]string, 0, len(mems))
		for _, m := range mems {
			items = append(items, fmt.Sprintf("- %s", m.Content))
		}
		sections = append(sections, fmt.Sprintf("[%s]\n%s", label, joinLines(items)))
	}

	if len(sections) == 0 {
		return ""
	}
	return joinLines(sections)
}

// IndexAll indexes all typed memories into a SemanticMemoryStore.
func (t *TypedMemoryStore) IndexAll(ctx context.Context, sem *SemanticMemoryStore) error {
	if sem == nil {
		return nil
	}
	for _, mt := range []MemoryType{MemoryTypeSemantic, MemoryTypeEpisodic, MemoryTypeProcedural} {
		mems, err := t.ListByType(mt)
		if err != nil {
			return err
		}
		for _, m := range mems {
			meta := map[string]string{
				"namespace": t.namespace,
				"type":      string(m.Type),
				"id":        m.ID,
			}
			for k, v := range m.Metadata {
				meta[k] = v
			}
			if err := sem.IndexMemory(ctx, m.ID, m.Content, meta); err != nil {
				return err
			}
		}
	}
	return nil
}

func memoryTypeLabel(mt MemoryType) string {
	switch mt {
	case MemoryTypeSemantic:
		return "事实与知识"
	case MemoryTypeEpisodic:
		return "重要经历"
	case MemoryTypeProcedural:
		return "行为规则"
	default:
		return string(mt)
	}
}

func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}
