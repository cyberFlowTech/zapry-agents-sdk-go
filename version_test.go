package agentsdk

import "testing"

func TestGetVersionInfo(t *testing.T) {
	info := GetVersionInfo()
	if info.Version == "" {
		t.Fatal("version should not be empty")
	}
	if info.GitCommit == "" {
		t.Fatal("git commit should not be empty")
	}
	if info.BuildTime == "" {
		t.Fatal("build time should not be empty")
	}
}
