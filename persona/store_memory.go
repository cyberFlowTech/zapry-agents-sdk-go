package persona

import (
	"fmt"
	"sync"

)

// InMemoryPersonaStore is a thread-safe in-memory PersonaStore for testing.
type InMemoryPersonaStore struct {
	mu       sync.RWMutex
	configs  map[string]map[string]*RuntimeConfig // persona_id → version → config
	byHash   map[string]*RuntimeConfig            // spec_hash → config
	latest   map[string]string                           // persona_id → latest version
}

// NewInMemoryPersonaStore creates a new in-memory 
func NewInMemoryPersonaStore() *InMemoryPersonaStore {
	return &InMemoryPersonaStore{
		configs: make(map[string]map[string]*RuntimeConfig),
		byHash:  make(map[string]*RuntimeConfig),
		latest:  make(map[string]string),
	}
}

func (s *InMemoryPersonaStore) Save(config *RuntimeConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.configs[config.PersonaID]; !ok {
		s.configs[config.PersonaID] = make(map[string]*RuntimeConfig)
	}

	if _, exists := s.configs[config.PersonaID][config.Version]; exists {
		return fmt.Errorf("version %s already exists for persona %s", config.Version, config.PersonaID)
	}

	s.configs[config.PersonaID][config.Version] = config
	s.byHash[config.SpecHash] = config
	s.latest[config.PersonaID] = config.Version
	return nil
}

func (s *InMemoryPersonaStore) Get(personaID string) (*RuntimeConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	version, ok := s.latest[personaID]
	if !ok {
		return nil, fmt.Errorf("persona %s not found", personaID)
	}
	return s.configs[personaID][version], nil
}

func (s *InMemoryPersonaStore) GetVersion(personaID string, version string) (*RuntimeConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, ok := s.configs[personaID]
	if !ok {
		return nil, fmt.Errorf("persona %s not found", personaID)
	}
	config, ok := versions[version]
	if !ok {
		return nil, fmt.Errorf("version %s not found for persona %s", version, personaID)
	}
	return config, nil
}

func (s *InMemoryPersonaStore) GetBySpecHash(specHash string) (*RuntimeConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	config, ok := s.byHash[specHash]
	if !ok {
		return nil, nil // not found is not an error (for idempotency check)
	}
	return config, nil
}

func (s *InMemoryPersonaStore) ListVersions(personaID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, ok := s.configs[personaID]
	if !ok {
		return nil, fmt.Errorf("persona %s not found", personaID)
	}
	result := make([]string, 0, len(versions))
	for v := range versions {
		result = append(result, v)
	}
	return result, nil
}
