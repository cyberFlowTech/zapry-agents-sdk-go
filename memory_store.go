package agentsdk

import (
	"sync"
)

// MemoryStore is the pluggable storage backend interface for the memory framework.
//
// All data is organized by namespace (typically "{agent_id}:{user_id}")
// and key (e.g. "long_term", "short_term").
type MemoryStore interface {
	// KV operations
	Get(namespace, key string) (string, error)
	Set(namespace, key, value string) error
	Delete(namespace, key string) error
	ListKeys(namespace string) ([]string, error)

	// List operations (ordered sequences for chat history, buffer)
	Append(namespace, key, value string) error
	GetList(namespace, key string, limit, offset int) ([]string, error)
	TrimList(namespace, key string, maxSize int) error
	ClearList(namespace, key string) error
	ListLength(namespace, key string) (int, error)
}

// InMemoryMemoryStore is a thread-safe in-memory MemoryStore for development.
// Data is lost on restart.
type InMemoryMemoryStore struct {
	mu    sync.RWMutex
	kv    map[string]map[string]string
	lists map[string]map[string][]string
}

// NewInMemoryMemoryStore creates a new in-memory store.
func NewInMemoryMemoryStore() *InMemoryMemoryStore {
	return &InMemoryMemoryStore{
		kv:    make(map[string]map[string]string),
		lists: make(map[string]map[string][]string),
	}
}

func (s *InMemoryMemoryStore) Get(namespace, key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if ns, ok := s.kv[namespace]; ok {
		if v, ok := ns[key]; ok {
			return v, nil
		}
	}
	return "", nil
}

func (s *InMemoryMemoryStore) Set(namespace, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.kv[namespace] == nil {
		s.kv[namespace] = make(map[string]string)
	}
	s.kv[namespace][key] = value
	return nil
}

func (s *InMemoryMemoryStore) Delete(namespace, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ns, ok := s.kv[namespace]; ok {
		delete(ns, key)
	}
	return nil
}

func (s *InMemoryMemoryStore) ListKeys(namespace string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := make(map[string]bool)
	if ns, ok := s.kv[namespace]; ok {
		for k := range ns {
			seen[k] = true
		}
	}
	if ns, ok := s.lists[namespace]; ok {
		for k := range ns {
			seen[k] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *InMemoryMemoryStore) Append(namespace, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lists[namespace] == nil {
		s.lists[namespace] = make(map[string][]string)
	}
	s.lists[namespace][key] = append(s.lists[namespace][key], value)
	return nil
}

func (s *InMemoryMemoryStore) GetList(namespace, key string, limit, offset int) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var items []string
	if ns, ok := s.lists[namespace]; ok {
		items = ns[key]
	}
	if items == nil {
		return []string{}, nil
	}
	if offset > 0 && offset < len(items) {
		items = items[offset:]
	} else if offset >= len(items) {
		return []string{}, nil
	}
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	result := make([]string, len(items))
	copy(result, items)
	return result, nil
}

func (s *InMemoryMemoryStore) TrimList(namespace, key string, maxSize int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ns, ok := s.lists[namespace]; ok {
		if lst, ok := ns[key]; ok && len(lst) > maxSize {
			ns[key] = lst[len(lst)-maxSize:]
		}
	}
	return nil
}

func (s *InMemoryMemoryStore) ClearList(namespace, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ns, ok := s.lists[namespace]; ok {
		ns[key] = nil
	}
	return nil
}

func (s *InMemoryMemoryStore) ListLength(namespace, key string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if ns, ok := s.lists[namespace]; ok {
		return len(ns[key]), nil
	}
	return 0, nil
}
