package agentsdk

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListBuiltinSkills(t *testing.T) {
	skills := ListBuiltinSkills()
	if len(skills) == 0 {
		t.Fatalf("expected builtin skills, got empty")
	}
	if skills[0].Key == "" {
		t.Fatalf("expected non-empty skill key")
	}
}

func TestBuildProfileSourceFromDirWithBuiltinSkills_WithoutLocalSkills(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "SOUL.md"), []byte("# Test Agent\n\nhello"), 0o600); err != nil {
		t.Fatalf("write SOUL.md failed: %v", err)
	}

	source, err := BuildProfileSourceFromDirWithBuiltinSkills(baseDir, "test-agent", "image-generation", "knowledge-qa")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if source == nil {
		t.Fatalf("expected non-nil source")
	}
	if len(source.Skills) != 2 {
		t.Fatalf("expected 2 builtin skills, got %d", len(source.Skills))
	}
	if source.SnapshotID == "" {
		t.Fatalf("expected snapshot id")
	}
}

func TestBuildProfileSourceFromDirWithBuiltinSkills_LocalOverridesBuiltin(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "SOUL.md"), []byte("# Agent"), 0o600); err != nil {
		t.Fatalf("write SOUL.md failed: %v", err)
	}
	localSkillDir := filepath.Join(baseDir, "skills", "image-generation")
	if err := os.MkdirAll(localSkillDir, 0o700); err != nil {
		t.Fatalf("mkdir local skill dir failed: %v", err)
	}
	localSkill := `---
skillKey: image-generation
skillVersion: 9.9.9
source: local
---

# Local override
`
	if err := os.WriteFile(filepath.Join(localSkillDir, "SKILL.md"), []byte(localSkill), 0o600); err != nil {
		t.Fatalf("write local SKILL.md failed: %v", err)
	}

	source, err := BuildProfileSourceFromDirWithBuiltinSkills(baseDir, "test-agent", "image-generation")
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if len(source.Skills) != 1 {
		t.Fatalf("expected only local skill to remain, got %d", len(source.Skills))
	}
	if source.Skills[0].SkillVersion != "9.9.9" {
		t.Fatalf("expected local override version 9.9.9, got %s", source.Skills[0].SkillVersion)
	}
}

func TestBuildProfileSourceFromDirWithBuiltinSkills_UnknownBuiltinSkill(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "SOUL.md"), []byte("# Agent"), 0o600); err != nil {
		t.Fatalf("write SOUL.md failed: %v", err)
	}
	_, err := BuildProfileSourceFromDirWithBuiltinSkills(baseDir, "test-agent", "unknown-skill")
	if err == nil {
		t.Fatalf("expected unknown builtin skill error")
	}
}

