package agentsdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
)

// ──────────────────────────────────────────────
// MCP Client — JSON-RPC 2.0 protocol layer
// ──────────────────────────────────────────────

// ── JSON-RPC 2.0 types ──

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPError is the unified protocol-level error type carrying JSON-RPC error code.
type MCPError struct {
	Code    int
	Message string
}

func (e *MCPError) Error() string {
	return fmt.Sprintf("mcp error %d: %s", e.Code, e.Message)
}

// ── MCP protocol types ──

// MCPToolDef represents a tool definition returned by MCP tools/list.
type MCPToolDef struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPToolResult represents the result of MCP tools/call.
type MCPToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError"`
}

// MCPContent is a single content block in an MCP tool result.
type MCPContent struct {
	Type string `json:"type"` // "text", "image", etc.
	Text string `json:"text,omitempty"`
}

// MCPInitResult is the response from MCP initialize.
type MCPInitResult struct {
	ProtocolVersion string        `json:"protocolVersion"`
	ServerInfo      MCPServerInfo `json:"serverInfo"`
}

// MCPServerInfo describes the MCP server identity.
type MCPServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ── MCPClient ──

// MCPClient wraps a transport and provides typed MCP protocol methods.
type MCPClient struct {
	transport MCPTransport
	nextID    atomic.Int64
}

// NewMCPClient creates a new MCP client over the given transport.
func NewMCPClient(transport MCPTransport) *MCPClient {
	return &MCPClient{transport: transport}
}

// call is the internal unified JSON-RPC call method.
func (c *MCPClient) call(ctx context.Context, method string, params interface{}, result interface{}) error {
	id := c.nextID.Add(1)
	req := jsonRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	payload, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("mcp: marshal request: %w", err)
	}

	respBytes, err := c.transport.Call(ctx, payload)
	if err != nil {
		return err
	}

	var resp jsonRPCResponse
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return fmt.Errorf("mcp: unmarshal response: %w", err)
	}

	if resp.Error != nil {
		return &MCPError{Code: resp.Error.Code, Message: resp.Error.Message}
	}

	if result != nil && resp.Result != nil {
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return fmt.Errorf("mcp: unmarshal result: %w", err)
		}
	}
	return nil
}

// Initialize performs the MCP handshake.
func (c *MCPClient) Initialize(ctx context.Context) (*MCPInitResult, error) {
	params := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]string{
			"name":    "zapry-agents-sdk-go",
			"version": "1.0.0",
		},
	}
	var result MCPInitResult
	if err := c.call(ctx, "initialize", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListTools discovers available tools from the MCP server.
// Handles both {tools:[...]} (standard) and bare [...] response formats.
func (c *MCPClient) ListTools(ctx context.Context) ([]MCPToolDef, error) {
	var raw json.RawMessage
	if err := c.call(ctx, "tools/list", nil, &raw); err != nil {
		return nil, err
	}

	trimmed := bytes.TrimSpace([]byte(raw))
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("mcp: empty tools/list response")
	}

	if trimmed[0] == '{' {
		var wrapped struct {
			Tools []MCPToolDef `json:"tools"`
		}
		if err := json.Unmarshal(trimmed, &wrapped); err != nil {
			return nil, fmt.Errorf("mcp: unmarshal tools/list object: %w", err)
		}
		return wrapped.Tools, nil
	}

	if trimmed[0] == '[' {
		var tools []MCPToolDef
		if err := json.Unmarshal(trimmed, &tools); err != nil {
			return nil, fmt.Errorf("mcp: unmarshal tools/list array: %w", err)
		}
		return tools, nil
	}

	return nil, fmt.Errorf("mcp: unexpected tools/list result format")
}

// CallTool invokes a tool on the MCP server.
func (c *MCPClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (*MCPToolResult, error) {
	params := map[string]interface{}{
		"name":      name,
		"arguments": args,
	}
	var result MCPToolResult
	if err := c.call(ctx, "tools/call", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Close closes the underlying transport.
func (c *MCPClient) Close() error {
	return c.transport.Close()
}
