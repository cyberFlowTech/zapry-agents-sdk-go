package agentsdk

import "path"

// ──────────────────────────────────────────────
// MCP Client — Configuration types
// ──────────────────────────────────────────────

// MCPServerConfig defines the connection configuration for a single MCP server.
type MCPServerConfig struct {
	Name      string // unique identifier (e.g. "filesystem")
	Transport string // "stdio" | "http"

	// Stdio configuration (Milestone 2)
	Command string
	Args    []string
	Env     map[string]string

	// HTTP configuration
	URL     string
	Headers map[string]string

	// General
	Timeout    int // seconds, default 30
	MaxRetries int // retry count for retryable errors, default 3 (only 5xx/network/timeout, not 4xx)

	// Tool filtering (matches original MCP tool name, NOT the injected sdk name).
	// Supports wildcards via path.Match: read_*, list_*, dangerous_*
	AllowedTools []string // whitelist; empty = allow all
	BlockedTools []string // blacklist
	MaxTools     int      // max tools to inject; 0 = no limit
}

// MCPManagerConfig provides manager-level configuration.
type MCPManagerConfig struct {
	ToolPrefix string // naming template, default "mcp.{server}.{tool}"
	TraceArgs  bool   // whether to record args/result in tracing spans, default false
}

// matchToolFilter checks if toolName matches a wildcard pattern (via path.Match).
// Supports * and ? wildcards.
func matchToolFilter(pattern, toolName string) bool {
	matched, err := path.Match(pattern, toolName)
	if err != nil {
		return false
	}
	return matched
}

// isToolAllowed checks whether an original MCP tool name passes the filter.
// BlockedTools takes precedence over AllowedTools.
func isToolAllowed(name string, config *MCPServerConfig) bool {
	if config == nil {
		return true
	}
	for _, p := range config.BlockedTools {
		if matchToolFilter(p, name) {
			return false
		}
	}
	if len(config.AllowedTools) == 0 {
		return true
	}
	for _, p := range config.AllowedTools {
		if matchToolFilter(p, name) {
			return true
		}
	}
	return false
}
