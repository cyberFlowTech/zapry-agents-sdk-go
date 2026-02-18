// Package store defines the storage abstraction for persona configs.
package persona

import (
)

// PersonaStore is the interface for persisting and retrieving RuntimeConfigs.
type PersonaStore interface {
	// Save persists a RuntimeConfig. If version already exists, returns error.
	Save(config *RuntimeConfig) error

	// Get retrieves the latest version of a persona config.
	Get(personaID string) (*RuntimeConfig, error)

	// GetVersion retrieves a specific version of a persona config.
	GetVersion(personaID string, version string) (*RuntimeConfig, error)

	// GetBySpecHash returns a config if one with the same spec_hash exists (idempotency).
	GetBySpecHash(specHash string) (*RuntimeConfig, error)

	// ListVersions returns all versions for a persona.
	ListVersions(personaID string) ([]string, error)
}
