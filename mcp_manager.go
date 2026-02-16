package agentsdk

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// MCP Client — MCPManager
// ──────────────────────────────────────────────

type mcpServerConn struct {
	config   MCPServerConfig
	client   *MCPClient
	mcpTools []MCPToolDef // raw MCP tool definitions
	sdkTools []*Tool      // converted SDK tools
}

// MCPManager manages multiple MCP server connections and injects their tools
// into a ToolRegistry for seamless use with AgentLoop.
type MCPManager struct {
	mu            sync.RWMutex
	servers       map[string]*mcpServerConn
	config        MCPManagerConfig
	toolMap       map[string]string // sdkToolName -> serverName (for routing CallTool)
	injectedTools []string          // tracks injected tool names for precise removal
}

// NewMCPManager creates a new MCP manager with optional configuration.
func NewMCPManager(config ...MCPManagerConfig) *MCPManager {
	cfg := MCPManagerConfig{}
	if len(config) > 0 {
		cfg = config[0]
	}
	if cfg.ToolPrefix == "" {
		cfg.ToolPrefix = "mcp.{server}.{tool}"
	}
	return &MCPManager{
		servers: make(map[string]*mcpServerConn),
		config:  cfg,
		toolMap: make(map[string]string),
	}
}

// AddServer connects to an MCP server: creates transport -> Start -> Initialize -> ListTools -> Convert.
func (m *MCPManager) AddServer(ctx context.Context, config MCPServerConfig) error {
	if config.Timeout <= 0 {
		config.Timeout = 30
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	timeout := time.Duration(config.Timeout) * time.Second

	var transport MCPTransport
	switch config.Transport {
	case "http":
		transport = NewHTTPTransport(config.URL, config.Headers, timeout)
	case "stdio":
		transport = NewStdioTransport(config.Command, config.Args, config.Env, timeout)
	default:
		// Allow custom transports passed via AddServerWithTransport
		return fmt.Errorf("mcp: unsupported transport: %q", config.Transport)
	}

	return m.addServerWithTransport(ctx, config, transport)
}

// AddServerWithTransport connects using a custom transport (useful for testing with InProcessTransport).
func (m *MCPManager) AddServerWithTransport(ctx context.Context, config MCPServerConfig, transport MCPTransport) error {
	if config.Timeout <= 0 {
		config.Timeout = 30
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = 3
	}
	return m.addServerWithTransport(ctx, config, transport)
}

func (m *MCPManager) addServerWithTransport(ctx context.Context, config MCPServerConfig, transport MCPTransport) error {
	if err := transport.Start(ctx); err != nil {
		return fmt.Errorf("mcp: start transport: %w", err)
	}

	client := NewMCPClient(transport)

	if _, err := client.Initialize(ctx); err != nil {
		transport.Close()
		return fmt.Errorf("mcp: initialize %q: %w", config.Name, err)
	}

	mcpTools, err := client.ListTools(ctx)
	if err != nil {
		transport.Close()
		return fmt.Errorf("mcp: list tools %q: %w", config.Name, err)
	}

	callFn := func(callCtx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
		return m.callToolDirect(callCtx, config.Name, toolName, args, config.MaxRetries)
	}
	sdkTools := ConvertMCPTools(config.Name, mcpTools, callFn, &config)

	m.mu.Lock()
	defer m.mu.Unlock()

	conn := &mcpServerConn{
		config:   config,
		client:   client,
		mcpTools: mcpTools,
		sdkTools: sdkTools,
	}
	m.servers[config.Name] = conn

	for _, t := range sdkTools {
		m.toolMap[t.Name] = config.Name
	}

	log.Printf("[MCPManager] Added server %q with %d tools", config.Name, len(sdkTools))
	return nil
}

// RemoveServer disconnects and removes a server and its tools.
func (m *MCPManager) RemoveServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn, ok := m.servers[name]
	if !ok {
		return fmt.Errorf("mcp: server %q not found", name)
	}

	for _, t := range conn.sdkTools {
		delete(m.toolMap, t.Name)
	}

	err := conn.client.Close()
	delete(m.servers, name)
	return err
}

// ── Tool Injection ──

// InjectTools registers all MCP tools into the ToolRegistry (idempotent: removes old tools first).
func (m *MCPManager) InjectTools(registry *ToolRegistry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove previously injected tools first (idempotent)
	for _, name := range m.injectedTools {
		registry.Remove(name)
	}
	m.injectedTools = nil

	for _, conn := range m.servers {
		for _, tool := range conn.sdkTools {
			registry.Register(tool)
			m.injectedTools = append(m.injectedTools, tool.Name)
		}
	}
}

// RemoveTools precisely removes only the MCP-injected tools from the registry.
func (m *MCPManager) RemoveTools(registry *ToolRegistry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, name := range m.injectedTools {
		registry.Remove(name)
	}
	m.injectedTools = nil
}

// ── Tool Invocation ──

// CallTool routes a call by SDK tool name to the correct server.
// This is typically called by the injected Tool's Handler closure.
func (m *MCPManager) CallTool(ctx context.Context, sdkToolName string, args map[string]interface{}) (interface{}, error) {
	m.mu.RLock()
	serverName, ok := m.toolMap[sdkToolName]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("mcp: tool %q not found", sdkToolName)
	}
	conn, ok := m.servers[serverName]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("mcp: server %q not found", serverName)
	}
	maxRetries := conn.config.MaxRetries
	m.mu.RUnlock()

	prefix := "mcp." + serverName + "."
	originalName := strings.TrimPrefix(sdkToolName, prefix)

	return m.callToolDirect(ctx, serverName, originalName, args, maxRetries)
}

// callToolDirect calls a specific server's tool with retry logic for retryable errors.
func (m *MCPManager) callToolDirect(ctx context.Context, serverName, toolName string, args map[string]interface{}, maxRetries int) (interface{}, error) {
	m.mu.RLock()
	conn, ok := m.servers[serverName]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("mcp: server %q not found", serverName)
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		result, err := conn.client.CallTool(ctx, toolName, args)
		if err != nil {
			lastErr = err
			var transportErr *MCPTransportError
			if errors.As(err, &transportErr) && transportErr.IsRetryable() {
				continue
			}
			return nil, err
		}

		cr := mcpResultToCallResult(result)
		return cr.Text, nil
	}

	return nil, fmt.Errorf("mcp: call %s.%s failed after %d retries: %w", serverName, toolName, maxRetries, lastErr)
}

// ── Refresh ──

// RefreshTools re-discovers tools for the specified (or all) servers.
func (m *MCPManager) RefreshTools(ctx context.Context, server ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	targets := server
	if len(targets) == 0 {
		for name := range m.servers {
			targets = append(targets, name)
		}
	}

	for _, name := range targets {
		conn, ok := m.servers[name]
		if !ok {
			continue
		}

		for _, t := range conn.sdkTools {
			delete(m.toolMap, t.Name)
		}

		mcpTools, err := conn.client.ListTools(ctx)
		if err != nil {
			return fmt.Errorf("mcp: refresh %q: %w", name, err)
		}

		callFn := func(callCtx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
			return m.callToolDirect(callCtx, name, toolName, args, conn.config.MaxRetries)
		}
		sdkTools := ConvertMCPTools(name, mcpTools, callFn, &conn.config)

		conn.mcpTools = mcpTools
		conn.sdkTools = sdkTools

		for _, t := range sdkTools {
			m.toolMap[t.Name] = name
		}
	}

	return nil
}

// ── Lifecycle ──

// DisconnectAll closes all server connections and clears internal state.
func (m *MCPManager) DisconnectAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []string
	for name, conn := range m.servers {
		if err := conn.client.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}
	m.servers = make(map[string]*mcpServerConn)
	m.toolMap = make(map[string]string)
	m.injectedTools = nil

	if len(errs) > 0 {
		return fmt.Errorf("mcp: disconnect errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

// ── Query ──

// ListTools returns all (or server-specific) converted SDK tools.
func (m *MCPManager) ListTools(server ...string) []*Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Tool
	if len(server) == 0 {
		for _, conn := range m.servers {
			result = append(result, conn.sdkTools...)
		}
	} else {
		for _, name := range server {
			if conn, ok := m.servers[name]; ok {
				result = append(result, conn.sdkTools...)
			}
		}
	}
	return result
}

// ServerNames returns the names of all connected servers.
func (m *MCPManager) ServerNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.servers))
	for n := range m.servers {
		names = append(names, n)
	}
	return names
}
