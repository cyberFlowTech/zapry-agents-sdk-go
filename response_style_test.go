package agentsdk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadForbiddenPhrasesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phrases.txt")
	content := "# comment\n\nfoo\nbar\nfoo\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write phrases file failed: %v", err)
	}

	phrases, err := LoadForbiddenPhrasesFile(path)
	if err != nil {
		t.Fatalf("load phrases file failed: %v", err)
	}
	if len(phrases) != 2 {
		t.Fatalf("expected 2 unique phrases, got %d (%v)", len(phrases), phrases)
	}
	if phrases[0] != "foo" || phrases[1] != "bar" {
		t.Fatalf("unexpected phrases: %v", phrases)
	}
}

func TestNewResponseStyleController_ForbiddenFileOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "phrases.txt")
	if err := os.WriteFile(path, []byte("custom phrase\n"), 0o600); err != nil {
		t.Fatalf("write phrases file failed: %v", err)
	}

	ctrl := NewResponseStyleController(StyleConfig{
		ForbiddenPhrasesFile: path,
	})
	out, changed, _ := ctrl.PostProcess("hello custom phrase world")
	if !changed {
		t.Fatal("expected forbidden phrase removal")
	}
	if strings.Contains(out, "custom phrase") {
		t.Fatalf("forbidden phrase should be removed, got: %s", out)
	}
}
