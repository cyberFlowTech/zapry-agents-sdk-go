package agentsdk

// Build/version variables can be injected via -ldflags:
// -X 'github.com/cyberFlowTech/zapry-agents-sdk-go.Version=v1.2.3'
// -X 'github.com/cyberFlowTech/zapry-agents-sdk-go.GitCommit=abc1234'
// -X 'github.com/cyberFlowTech/zapry-agents-sdk-go.BuildTime=2026-03-05T10:00:00Z'
var (
	Version   = "v0.0.0-dev"
	GitCommit = "unknown"
	BuildTime = "unknown"
)

// VersionInfo describes the runtime/build identity of this SDK.
type VersionInfo struct {
	Version   string `json:"version"`
	GitCommit string `json:"git_commit"`
	BuildTime string `json:"build_time"`
}

// GetVersionInfo returns the current SDK version metadata.
func GetVersionInfo() VersionInfo {
	return VersionInfo{
		Version:   Version,
		GitCommit: GitCommit,
		BuildTime: BuildTime,
	}
}
