package persona

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

)

// FileStore persists RuntimeConfigs as JSON files on disk.
// Layout: {baseDir}/{persona_id}/{version}.json
//
//	{baseDir}/{persona_id}/latest → symlink or version string
type FileStore struct {
	BaseDir string
}

// NewFileStore creates a FileStore at the given directory.
func NewFileStore(baseDir string) *FileStore {
	return &FileStore{BaseDir: baseDir}
}

func (s *FileStore) Save(config *RuntimeConfig) error {
	dir := filepath.Join(s.BaseDir, config.PersonaID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}

	// Check if version already exists
	versionPath := filepath.Join(dir, config.Version+".json")
	if _, err := os.Stat(versionPath); err == nil {
		return fmt.Errorf("version %s already exists for persona %s", config.Version, config.PersonaID)
	}

	// Write version file
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := os.WriteFile(versionPath, data, 0644); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	// Update latest pointer
	latestPath := filepath.Join(dir, "latest")
	if err := os.WriteFile(latestPath, []byte(config.Version), 0644); err != nil {
		return fmt.Errorf("write latest: %w", err)
	}

	// Write spec_hash index
	hashDir := filepath.Join(s.BaseDir, ".hash_index")
	os.MkdirAll(hashDir, 0755)
	hashPath := filepath.Join(hashDir, config.SpecHash)
	ref := fmt.Sprintf("%s/%s", config.PersonaID, config.Version)
	os.WriteFile(hashPath, []byte(ref), 0644)

	return nil
}

func (s *FileStore) Get(personaID string) (*RuntimeConfig, error) {
	dir := filepath.Join(s.BaseDir, personaID)
	latestPath := filepath.Join(dir, "latest")
	versionBytes, err := os.ReadFile(latestPath)
	if err != nil {
		return nil, fmt.Errorf("persona %s not found: %w", personaID, err)
	}
	return s.GetVersion(personaID, string(versionBytes))
}

func (s *FileStore) GetVersion(personaID string, version string) (*RuntimeConfig, error) {
	path := filepath.Join(s.BaseDir, personaID, version+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("version %s not found for persona %s: %w", version, personaID, err)
	}
	var config RuntimeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &config, nil
}

func (s *FileStore) GetBySpecHash(specHash string) (*RuntimeConfig, error) {
	hashPath := filepath.Join(s.BaseDir, ".hash_index", specHash)
	refBytes, err := os.ReadFile(hashPath)
	if err != nil {
		return nil, nil // not found
	}
	ref := string(refBytes)
	// Parse "persona_id/version"
	var personaID, version string
	for i := range ref {
		if ref[i] == '/' {
			personaID = ref[:i]
			version = ref[i+1:]
			break
		}
	}
	if personaID == "" {
		return nil, nil
	}
	return s.GetVersion(personaID, version)
}

func (s *FileStore) ListVersions(personaID string) ([]string, error) {
	dir := filepath.Join(s.BaseDir, personaID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("persona %s not found: %w", personaID, err)
	}
	var versions []string
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) == ".json" {
			versions = append(versions, name[:len(name)-5])
		}
	}
	sort.Strings(versions)
	return versions, nil
}
