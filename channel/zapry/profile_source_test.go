package zapry

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildProfileSourceFromDir_SkillsDirectoryNotFound(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "SOUL.md"), []byte("# Agent"), 0o600); err != nil {
		t.Fatalf("write SOUL.md failed: %v", err)
	}

	_, err := BuildProfileSourceFromDir(baseDir, "agent-a")
	if err == nil {
		t.Fatal("expected skills directory error")
	}
	if !errors.Is(err, ErrSkillsDirectoryNotFound) {
		t.Fatalf("expected ErrSkillsDirectoryNotFound, got %v", err)
	}
}

func TestBuildProfileSourceFromDir_NoSkillMarkdownFound(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "SOUL.md"), []byte("# Agent"), 0o600); err != nil {
		t.Fatalf("write SOUL.md failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(baseDir, "skills"), 0o700); err != nil {
		t.Fatalf("mkdir skills failed: %v", err)
	}

	_, err := BuildProfileSourceFromDir(baseDir, "agent-a")
	if err == nil {
		t.Fatal("expected no skill markdown error")
	}
	if !errors.Is(err, ErrNoSkillMarkdownFound) {
		t.Fatalf("expected ErrNoSkillMarkdownFound, got %v", err)
	}
}
