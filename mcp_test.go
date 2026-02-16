package agentsdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ══════════════════════════════════════════════
// Test helpers — mock MCP server via InProcessTransport
// ══════════════════════════════════════════════

// newMockMCPTransport creates an InProcessTransport that simulates an MCP server.
// tools defines the tool list returned by tools/list.
// callHandler is invoked for tools/call requests.
func newMockMCPTransport(
	tools []MCPToolDef,
	callHandler func(name string, args map[string]interface{}) (*MCPToolResult, error),
) *InProcessTransport {
	return NewInProcessTransport(func(request []byte) ([]byte, error) {
		var req jsonRPCRequest
		if err := json.Unmarshal(request, &req); err != nil {
			return json.Marshal(jsonRPCResponse{
				JSONRPC: "2.0", ID: 0,
				Error: &jsonRPCError{Code: -32700, Message: "parse error"},
			})
		}

		switch req.Method {
		case "initialize":
			result := MCPInitResult{
				ProtocolVersion: "2024-11-05",
				ServerInfo:      MCPServerInfo{Name: "mock", Version: "1.0"},
			}
			resultBytes, _ := json.Marshal(result)
			return json.Marshal(jsonRPCResponse{
				JSONRPC: "2.0", ID: req.ID,
				Result: json.RawMessage(resultBytes),
			})

		case "tools/list":
			wrapped := struct {
				Tools []MCPToolDef `json:"tools"`
			}{Tools: tools}
			resultBytes, _ := json.Marshal(wrapped)
			return json.Marshal(jsonRPCResponse{
				JSONRPC: "2.0", ID: req.ID,
				Result: json.RawMessage(resultBytes),
			})

		case "tools/call":
			var params struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			}
			paramsBytes, _ := json.Marshal(req.Params)
			json.Unmarshal(paramsBytes, &params)

			if callHandler == nil {
				return json.Marshal(jsonRPCResponse{
					JSONRPC: "2.0", ID: req.ID,
					Error: &jsonRPCError{Code: -1, Message: "no handler"},
				})
			}

			result, err := callHandler(params.Name, params.Arguments)
			if err != nil {
				return json.Marshal(jsonRPCResponse{
					JSONRPC: "2.0", ID: req.ID,
					Error: &jsonRPCError{Code: -1, Message: err.Error()},
				})
			}
			resultBytes, _ := json.Marshal(result)
			return json.Marshal(jsonRPCResponse{
				JSONRPC: "2.0", ID: req.ID,
				Result: json.RawMessage(resultBytes),
			})

		default:
			return json.Marshal(jsonRPCResponse{
				JSONRPC: "2.0", ID: req.ID,
				Error: &jsonRPCError{Code: -32601, Message: "method not found"},
			})
		}
	})
}

// helper: create a mock MCP server with standard tools
func standardMockTools() []MCPToolDef {
	return []MCPToolDef{
		{
			Name:        "read_file",
			Description: "Read contents of a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "File path"},
				},
				"required": []interface{}{"path"},
			},
		},
		{
			Name:        "list_files",
			Description: "List files in directory",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"dir": map[string]interface{}{"type": "string"},
				},
			},
		},
		{
			Name:        "write_file",
			Description: "Write to a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string"},
					"content": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"path", "content"},
			},
		},
	}
}

func standardCallHandler(name string, args map[string]interface{}) (*MCPToolResult, error) {
	switch name {
	case "read_file":
		path := args["path"].(string)
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: fmt.Sprintf("contents of %s", path)}},
		}, nil
	case "list_files":
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "file1.txt\nfile2.txt"}},
		}, nil
	case "write_file":
		return &MCPToolResult{
			Content: []MCPContent{{Type: "text", Text: "ok"}},
		}, nil
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// ══════════════════════════════════════════════
// Protocol layer tests
// ══════════════════════════════════════════════

func TestJSONRPCRequest_Marshal(t *testing.T) {
	req := jsonRPCRequest{JSONRPC: "2.0", ID: 1, Method: "test", Params: map[string]string{"a": "b"}}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"jsonrpc":"2.0"`) {
		t.Fatalf("missing jsonrpc field: %s", data)
	}
	if !strings.Contains(string(data), `"method":"test"`) {
		t.Fatalf("missing method: %s", data)
	}
}

func TestJSONRPCResponse_Unmarshal(t *testing.T) {
	raw := `{"jsonrpc":"2.0","id":1,"result":{"key":"value"}}`
	var resp jsonRPCResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID != 1 {
		t.Fatalf("expected id=1, got %d", resp.ID)
	}
	if resp.Error != nil {
		t.Fatal("expected no error")
	}
}

func TestJSONRPCResponse_Error(t *testing.T) {
	raw := `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"invalid request"}}`
	var resp jsonRPCResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != -32600 {
		t.Fatalf("expected code=-32600, got %d", resp.Error.Code)
	}
}

func TestMCPClient_Initialize(t *testing.T) {
	transport := newMockMCPTransport(nil, nil)
	client := NewMCPClient(transport)
	result, err := client.Initialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.ProtocolVersion != "2024-11-05" {
		t.Fatalf("expected protocol version 2024-11-05, got %s", result.ProtocolVersion)
	}
	if result.ServerInfo.Name != "mock" {
		t.Fatalf("expected server name=mock, got %s", result.ServerInfo.Name)
	}
}

func TestMCPClient_ListTools_WrappedFormat(t *testing.T) {
	tools := standardMockTools()
	transport := newMockMCPTransport(tools, nil)
	client := NewMCPClient(transport)
	client.Initialize(context.Background())

	result, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(result))
	}
	if result[0].Name != "read_file" {
		t.Fatalf("expected first tool=read_file, got %s", result[0].Name)
	}
}

func TestMCPClient_ListTools_BareArray(t *testing.T) {
	tools := []MCPToolDef{
		{Name: "search", Description: "Search", InputSchema: map[string]interface{}{"type": "object"}},
	}
	// Return bare array instead of wrapped
	transport := NewInProcessTransport(func(request []byte) ([]byte, error) {
		var req jsonRPCRequest
		json.Unmarshal(request, &req)
		switch req.Method {
		case "initialize":
			result := MCPInitResult{ProtocolVersion: "2024-11-05", ServerInfo: MCPServerInfo{Name: "bare", Version: "1.0"}}
			rb, _ := json.Marshal(result)
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)})
		case "tools/list":
			rb, _ := json.Marshal(tools) // bare array
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)})
		default:
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -1, Message: "nope"}})
		}
	})
	client := NewMCPClient(transport)
	client.Initialize(context.Background())

	result, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 || result[0].Name != "search" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestMCPClient_ListTools_EmptyArray(t *testing.T) {
	transport := newMockMCPTransport([]MCPToolDef{}, nil)
	client := NewMCPClient(transport)
	client.Initialize(context.Background())

	result, err := client.ListTools(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 tools, got %d", len(result))
	}
}

func TestMCPClient_CallTool_Success(t *testing.T) {
	transport := newMockMCPTransport(standardMockTools(), standardCallHandler)
	client := NewMCPClient(transport)
	client.Initialize(context.Background())

	result, err := client.CallTool(context.Background(), "read_file", map[string]interface{}{"path": "/tmp/test.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatal("unexpected isError=true")
	}
	if len(result.Content) != 1 || result.Content[0].Text != "contents of /tmp/test.txt" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestMCPClient_CallTool_InvalidJSON(t *testing.T) {
	transport := NewInProcessTransport(func(request []byte) ([]byte, error) {
		return []byte("not json"), nil
	})
	client := NewMCPClient(transport)
	_, err := client.CallTool(context.Background(), "test", nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Fatalf("expected unmarshal error, got: %v", err)
	}
}

func TestMCPClient_CallTool_MCPError(t *testing.T) {
	transport := NewInProcessTransport(func(request []byte) ([]byte, error) {
		var req jsonRPCRequest
		json.Unmarshal(request, &req)
		return json.Marshal(jsonRPCResponse{
			JSONRPC: "2.0", ID: req.ID,
			Error: &jsonRPCError{Code: -32000, Message: "server error"},
		})
	})
	client := NewMCPClient(transport)
	_, err := client.CallTool(context.Background(), "test", nil)
	if err == nil {
		t.Fatal("expected MCPError")
	}
	var mcpErr *MCPError
	if !errors.As(err, &mcpErr) {
		t.Fatalf("expected *MCPError, got %T: %v", err, err)
	}
	if mcpErr.Code != -32000 {
		t.Fatalf("expected code=-32000, got %d", mcpErr.Code)
	}
}

// ══════════════════════════════════════════════
// Converter tests
// ══════════════════════════════════════════════

func TestConvertMCPTools_Basic(t *testing.T) {
	mcpTools := standardMockTools()
	callFn := func(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
		return "result", nil
	}
	tools := ConvertMCPTools("fs", mcpTools, callFn, nil)
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}
	if tools[0].Name != "mcp.fs.read_file" {
		t.Fatalf("expected mcp.fs.read_file, got %s", tools[0].Name)
	}
	if !strings.Contains(tools[0].Description, "[MCP:fs]") {
		t.Fatalf("expected description to contain [MCP:fs], got %s", tools[0].Description)
	}
}

func TestConvertMCPTools_RawSchemaPreserved(t *testing.T) {
	mcpTools := standardMockTools()
	callFn := func(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
		return "ok", nil
	}
	tools := ConvertMCPTools("fs", mcpTools, callFn, nil)

	tool := tools[0] // read_file
	if tool.RawJSONSchema == nil {
		t.Fatal("RawJSONSchema should not be nil")
	}
	props := tool.RawJSONSchema["properties"].(map[string]interface{})
	if _, ok := props["path"]; !ok {
		t.Fatal("RawJSONSchema should contain 'path' property")
	}

	// Verify ToJSONSchema uses raw schema
	schema := tool.ToJSONSchema()
	params := schema["parameters"].(map[string]interface{})
	if params["type"] != "object" {
		t.Fatalf("expected parameters.type=object, got %v", params["type"])
	}
}

func TestConvertMCPTools_ExtractParams(t *testing.T) {
	mcpTools := standardMockTools()
	callFn := func(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
		return "ok", nil
	}
	tools := ConvertMCPTools("fs", mcpTools, callFn, nil)

	// read_file has 1 param: path (required)
	if len(tools[0].Parameters) != 1 {
		t.Fatalf("expected 1 param, got %d", len(tools[0].Parameters))
	}
	if tools[0].Parameters[0].Name != "path" {
		t.Fatalf("expected param name=path, got %s", tools[0].Parameters[0].Name)
	}
}

func TestConvertMCPTools_Required(t *testing.T) {
	mcpTools := standardMockTools()
	callFn := func(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
		return "ok", nil
	}
	tools := ConvertMCPTools("fs", mcpTools, callFn, nil)

	// read_file: path is required
	if !tools[0].Parameters[0].Required {
		t.Fatal("read_file.path should be required")
	}

	// list_files: dir is NOT required
	found := false
	for _, p := range tools[1].Parameters {
		if p.Name == "dir" {
			found = true
			if p.Required {
				t.Fatal("list_files.dir should not be required")
			}
		}
	}
	if !found {
		t.Fatal("list_files should have 'dir' param")
	}
}

func TestConvertMCPTools_AllowedFilter(t *testing.T) {
	mcpTools := standardMockTools() // read_file, list_files, write_file
	callFn := func(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
		return "ok", nil
	}
	config := &MCPServerConfig{AllowedTools: []string{"read_*"}}
	tools := ConvertMCPTools("fs", mcpTools, callFn, config)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool (read_file only), got %d", len(tools))
	}
	if tools[0].Name != "mcp.fs.read_file" {
		t.Fatalf("expected mcp.fs.read_file, got %s", tools[0].Name)
	}
}

func TestConvertMCPTools_BlockedFilter(t *testing.T) {
	mcpTools := standardMockTools()
	callFn := func(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
		return "ok", nil
	}
	config := &MCPServerConfig{BlockedTools: []string{"write_*"}}
	tools := ConvertMCPTools("fs", mcpTools, callFn, config)
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools (write_file blocked), got %d", len(tools))
	}
	for _, tool := range tools {
		if strings.Contains(tool.Name, "write") {
			t.Fatalf("write_file should be blocked, got %s", tool.Name)
		}
	}
}

func TestConvertMCPTools_MaxTools(t *testing.T) {
	mcpTools := standardMockTools() // 3 tools
	callFn := func(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
		return "ok", nil
	}
	config := &MCPServerConfig{MaxTools: 2}
	tools := ConvertMCPTools("fs", mcpTools, callFn, config)
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools (MaxTools=2), got %d", len(tools))
	}
}

func TestMCPResultToCallResult(t *testing.T) {
	tests := []struct {
		name     string
		result   *MCPToolResult
		expected string
		isError  bool
	}{
		{
			name: "single text",
			result: &MCPToolResult{
				Content: []MCPContent{{Type: "text", Text: "hello"}},
			},
			expected: "hello",
		},
		{
			name: "multiple text blocks",
			result: &MCPToolResult{
				Content: []MCPContent{
					{Type: "text", Text: "line1"},
					{Type: "text", Text: "line2"},
				},
			},
			expected: "line1\nline2",
		},
		{
			name: "isError wraps with prefix",
			result: &MCPToolResult{
				Content: []MCPContent{{Type: "text", Text: "something failed"}},
				IsError: true,
			},
			expected: "Error: something failed",
			isError:  true,
		},
		{
			name: "non-text content ignored",
			result: &MCPToolResult{
				Content: []MCPContent{
					{Type: "image", Text: ""},
					{Type: "text", Text: "only this"},
				},
			},
			expected: "only this",
		},
		{
			name:     "empty content",
			result:   &MCPToolResult{Content: []MCPContent{}},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cr := mcpResultToCallResult(tt.result)
			if cr.Text != tt.expected {
				t.Fatalf("expected text=%q, got %q", tt.expected, cr.Text)
			}
			if cr.Raw != tt.result {
				t.Fatal("raw should be the original result")
			}
		})
	}
}

// ══════════════════════════════════════════════
// MCPManager tests
// ══════════════════════════════════════════════

func addMockServer(t *testing.T, mgr *MCPManager, name string, tools []MCPToolDef, handler func(string, map[string]interface{}) (*MCPToolResult, error)) {
	t.Helper()
	transport := newMockMCPTransport(tools, handler)
	err := mgr.AddServerWithTransport(context.Background(), MCPServerConfig{Name: name, Transport: "custom"}, transport)
	if err != nil {
		t.Fatalf("AddServer(%s) failed: %v", name, err)
	}
}

func TestMCPManager_AddServer(t *testing.T) {
	mgr := NewMCPManager()
	addMockServer(t, mgr, "fs", standardMockTools(), standardCallHandler)

	names := mgr.ServerNames()
	if len(names) != 1 || names[0] != "fs" {
		t.Fatalf("expected [fs], got %v", names)
	}
	tools := mgr.ListTools()
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}
}

func TestMCPManager_RemoveServer(t *testing.T) {
	mgr := NewMCPManager()
	addMockServer(t, mgr, "fs", standardMockTools(), standardCallHandler)

	if err := mgr.RemoveServer("fs"); err != nil {
		t.Fatal(err)
	}
	if len(mgr.ServerNames()) != 0 {
		t.Fatal("expected 0 servers after remove")
	}
	if len(mgr.ListTools()) != 0 {
		t.Fatal("expected 0 tools after remove")
	}
}

func TestMCPManager_InjectTools(t *testing.T) {
	mgr := NewMCPManager()
	addMockServer(t, mgr, "fs", standardMockTools(), standardCallHandler)

	registry := NewToolRegistry()
	mgr.InjectTools(registry)

	if registry.Len() != 3 {
		t.Fatalf("expected 3 tools in registry, got %d", registry.Len())
	}
	if !registry.Contains("mcp.fs.read_file") {
		t.Fatal("registry should contain mcp.fs.read_file")
	}
}

func TestMCPManager_InjectTools_Idempotent(t *testing.T) {
	mgr := NewMCPManager()
	addMockServer(t, mgr, "fs", standardMockTools(), standardCallHandler)

	registry := NewToolRegistry()

	// Inject twice
	mgr.InjectTools(registry)
	mgr.InjectTools(registry)

	if registry.Len() != 3 {
		t.Fatalf("expected 3 tools after idempotent inject, got %d", registry.Len())
	}
}

func TestMCPManager_RemoveTools_Precise(t *testing.T) {
	mgr := NewMCPManager()
	addMockServer(t, mgr, "fs", standardMockTools(), standardCallHandler)

	registry := NewToolRegistry()

	// Register a user tool with a name that doesn't conflict
	registry.Register(&Tool{Name: "my_local_tool", Description: "local"})

	mgr.InjectTools(registry)

	// Registry should have 3 MCP + 1 local = 4
	if registry.Len() != 4 {
		t.Fatalf("expected 4 tools, got %d", registry.Len())
	}

	mgr.RemoveTools(registry)

	// Only local tool should remain
	if registry.Len() != 1 {
		t.Fatalf("expected 1 tool after RemoveTools, got %d", registry.Len())
	}
	if !registry.Contains("my_local_tool") {
		t.Fatal("local tool should not be removed")
	}
}

func TestMCPManager_CallTool_E2E(t *testing.T) {
	mgr := NewMCPManager()
	addMockServer(t, mgr, "fs", standardMockTools(), standardCallHandler)

	registry := NewToolRegistry()
	mgr.InjectTools(registry)

	// Execute through ToolRegistry (as AgentLoop would)
	ctx := &ToolContext{Ctx: context.Background()}
	result, err := registry.Execute("mcp.fs.read_file", map[string]interface{}{"path": "/tmp/data.txt"}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "contents of /tmp/data.txt" {
		t.Fatalf("expected file contents, got %v", result)
	}
}

func TestMCPManager_MultiServer(t *testing.T) {
	mgr := NewMCPManager()
	addMockServer(t, mgr, "fs", standardMockTools(), standardCallHandler)
	addMockServer(t, mgr, "db", []MCPToolDef{
		{Name: "query", Description: "Run SQL", InputSchema: map[string]interface{}{"type": "object"}},
	}, func(name string, args map[string]interface{}) (*MCPToolResult, error) {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: "rows:3"}}}, nil
	})

	if len(mgr.ServerNames()) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(mgr.ServerNames()))
	}

	tools := mgr.ListTools()
	if len(tools) != 4 { // 3 from fs + 1 from db
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}

	registry := NewToolRegistry()
	mgr.InjectTools(registry)

	// Call fs tool
	ctx := &ToolContext{Ctx: context.Background()}
	r1, err := registry.Execute("mcp.fs.read_file", map[string]interface{}{"path": "/x"}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r1 != "contents of /x" {
		t.Fatalf("unexpected fs result: %v", r1)
	}

	// Call db tool
	r2, err := registry.Execute("mcp.db.query", map[string]interface{}{}, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r2 != "rows:3" {
		t.Fatalf("unexpected db result: %v", r2)
	}
}

func TestMCPManager_ToolNameConflict(t *testing.T) {
	mgr := NewMCPManager()
	// Both servers have a tool named "read_file"
	addMockServer(t, mgr, "server1", []MCPToolDef{
		{Name: "read_file", Description: "Server1 read", InputSchema: map[string]interface{}{"type": "object"}},
	}, func(name string, args map[string]interface{}) (*MCPToolResult, error) {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: "from-server1"}}}, nil
	})
	addMockServer(t, mgr, "server2", []MCPToolDef{
		{Name: "read_file", Description: "Server2 read", InputSchema: map[string]interface{}{"type": "object"}},
	}, func(name string, args map[string]interface{}) (*MCPToolResult, error) {
		return &MCPToolResult{Content: []MCPContent{{Type: "text", Text: "from-server2"}}}, nil
	})

	registry := NewToolRegistry()
	mgr.InjectTools(registry)

	// Both should exist with different prefixes
	if !registry.Contains("mcp.server1.read_file") {
		t.Fatal("should have mcp.server1.read_file")
	}
	if !registry.Contains("mcp.server2.read_file") {
		t.Fatal("should have mcp.server2.read_file")
	}

	ctx := &ToolContext{Ctx: context.Background()}
	r1, _ := registry.Execute("mcp.server1.read_file", map[string]interface{}{}, ctx)
	r2, _ := registry.Execute("mcp.server2.read_file", map[string]interface{}{}, ctx)
	if r1 != "from-server1" || r2 != "from-server2" {
		t.Fatalf("routing failed: r1=%v, r2=%v", r1, r2)
	}
}

func TestMCPManager_RefreshTools(t *testing.T) {
	callCount := 0
	tools := []MCPToolDef{
		{Name: "tool_v1", Description: "V1", InputSchema: map[string]interface{}{"type": "object"}},
	}

	// Transport that changes tools after first list
	transport := NewInProcessTransport(func(request []byte) ([]byte, error) {
		var req jsonRPCRequest
		json.Unmarshal(request, &req)
		switch req.Method {
		case "initialize":
			r := MCPInitResult{ProtocolVersion: "2024-11-05", ServerInfo: MCPServerInfo{Name: "dyn", Version: "1.0"}}
			rb, _ := json.Marshal(r)
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)})
		case "tools/list":
			callCount++
			if callCount > 1 {
				tools = []MCPToolDef{
					{Name: "tool_v2", Description: "V2", InputSchema: map[string]interface{}{"type": "object"}},
					{Name: "tool_v3", Description: "V3", InputSchema: map[string]interface{}{"type": "object"}},
				}
			}
			wrapped := struct{ Tools []MCPToolDef }{Tools: tools}
			rb, _ := json.Marshal(wrapped)
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)})
		default:
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -1, Message: "nope"}})
		}
	})

	mgr := NewMCPManager()
	err := mgr.AddServerWithTransport(context.Background(), MCPServerConfig{Name: "dyn"}, transport)
	if err != nil {
		t.Fatal(err)
	}

	if len(mgr.ListTools()) != 1 {
		t.Fatalf("expected 1 tool initially, got %d", len(mgr.ListTools()))
	}

	err = mgr.RefreshTools(context.Background(), "dyn")
	if err != nil {
		t.Fatal(err)
	}

	toolList := mgr.ListTools()
	if len(toolList) != 2 {
		t.Fatalf("expected 2 tools after refresh, got %d", len(toolList))
	}
}

func TestMCPManager_DisconnectAll(t *testing.T) {
	mgr := NewMCPManager()
	addMockServer(t, mgr, "a", standardMockTools(), standardCallHandler)
	addMockServer(t, mgr, "b", standardMockTools(), standardCallHandler)

	if err := mgr.DisconnectAll(); err != nil {
		t.Fatal(err)
	}
	if len(mgr.ServerNames()) != 0 {
		t.Fatal("expected 0 servers after DisconnectAll")
	}
}

func TestMCPManager_ServerNotFound(t *testing.T) {
	mgr := NewMCPManager()
	_, err := mgr.CallTool(context.Background(), "mcp.noexist.tool", nil)
	if err == nil {
		t.Fatal("expected error for non-existent tool")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got: %v", err)
	}
}

func TestMCPManager_CallTool_Timeout(t *testing.T) {
	// Slow transport that exceeds context deadline
	transport := NewInProcessTransport(func(request []byte) ([]byte, error) {
		var req jsonRPCRequest
		json.Unmarshal(request, &req)
		switch req.Method {
		case "initialize":
			r := MCPInitResult{ProtocolVersion: "2024-11-05", ServerInfo: MCPServerInfo{Name: "slow", Version: "1.0"}}
			rb, _ := json.Marshal(r)
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)})
		case "tools/list":
			wrapped := struct{ Tools []MCPToolDef }{Tools: []MCPToolDef{
				{Name: "slow_tool", Description: "Slow", InputSchema: map[string]interface{}{"type": "object"}},
			}}
			rb, _ := json.Marshal(wrapped)
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)})
		case "tools/call":
			time.Sleep(500 * time.Millisecond) // slow
			r := MCPToolResult{Content: []MCPContent{{Type: "text", Text: "done"}}}
			rb, _ := json.Marshal(r)
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)})
		default:
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -1, Message: "nope"}})
		}
	})

	mgr := NewMCPManager()
	mgr.AddServerWithTransport(context.Background(), MCPServerConfig{Name: "slow", MaxRetries: 0}, transport)

	registry := NewToolRegistry()
	mgr.InjectTools(registry)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	toolCtx := &ToolContext{Ctx: ctx}
	_, err := registry.Execute("mcp.slow.slow_tool", map[string]interface{}{}, toolCtx)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestMCPManager_CallTool_Cancel(t *testing.T) {
	transport := NewInProcessTransport(func(request []byte) ([]byte, error) {
		var req jsonRPCRequest
		json.Unmarshal(request, &req)
		switch req.Method {
		case "initialize":
			r := MCPInitResult{ProtocolVersion: "2024-11-05", ServerInfo: MCPServerInfo{Name: "c", Version: "1.0"}}
			rb, _ := json.Marshal(r)
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)})
		case "tools/list":
			wrapped := struct{ Tools []MCPToolDef }{Tools: []MCPToolDef{
				{Name: "my_tool", Description: "Test", InputSchema: map[string]interface{}{"type": "object"}},
			}}
			rb, _ := json.Marshal(wrapped)
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)})
		case "tools/call":
			time.Sleep(1 * time.Second)
			r := MCPToolResult{Content: []MCPContent{{Type: "text", Text: "done"}}}
			rb, _ := json.Marshal(r)
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)})
		default:
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -1, Message: "nope"}})
		}
	})

	mgr := NewMCPManager()
	mgr.AddServerWithTransport(context.Background(), MCPServerConfig{Name: "c", MaxRetries: 0}, transport)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := mgr.CallTool(ctx, "mcp.c.my_tool", nil)
	if err == nil {
		t.Fatal("expected cancel error")
	}
}

func TestMCPManager_CallTool_Retry(t *testing.T) {
	attempts := 0
	transport := NewInProcessTransport(func(request []byte) ([]byte, error) {
		var req jsonRPCRequest
		json.Unmarshal(request, &req)
		switch req.Method {
		case "initialize":
			r := MCPInitResult{ProtocolVersion: "2024-11-05", ServerInfo: MCPServerInfo{Name: "r", Version: "1.0"}}
			rb, _ := json.Marshal(r)
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)})
		case "tools/list":
			wrapped := struct{ Tools []MCPToolDef }{Tools: []MCPToolDef{
				{Name: "flaky", Description: "Flaky", InputSchema: map[string]interface{}{"type": "object"}},
			}}
			rb, _ := json.Marshal(wrapped)
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)})
		case "tools/call":
			attempts++
			if attempts < 3 {
				// Simulate 503
				return nil, &MCPTransportError{StatusCode: 503, BodyPreview: "service unavailable"}
			}
			r := MCPToolResult{Content: []MCPContent{{Type: "text", Text: "success after retries"}}}
			rb, _ := json.Marshal(r)
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)})
		default:
			return json.Marshal(jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -1, Message: "nope"}})
		}
	})

	mgr := NewMCPManager()
	mgr.AddServerWithTransport(context.Background(), MCPServerConfig{Name: "r", MaxRetries: 5}, transport)

	result, err := mgr.CallTool(context.Background(), "mcp.r.flaky", nil)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if result != "success after retries" {
		t.Fatalf("unexpected result: %v", result)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

// ══════════════════════════════════════════════
// Integration tests
// ══════════════════════════════════════════════

func TestAgentLoop_MCPTool_Selected(t *testing.T) {
	mgr := NewMCPManager()
	addMockServer(t, mgr, "fs", standardMockTools(), standardCallHandler)

	registry := NewToolRegistry()
	mgr.InjectTools(registry)

	// LLM that always calls mcp.fs.read_file
	callNum := 0
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		callNum++
		if callNum == 1 {
			return makeToolCallResp([]struct{ Name, Args string }{
				{"mcp.fs.read_file", `{"path":"/tmp/hello.txt"}`},
			}, ""), nil
		}
		return makeFinalResp("File contents: contents of /tmp/hello.txt"), nil
	}

	loop := NewAgentLoop(llm, registry, "sys", 10, nil)
	result := loop.Run("Read /tmp/hello.txt", nil, "")

	if result.StoppedReason != "completed" {
		t.Fatalf("expected completed, got %s", result.StoppedReason)
	}
	if result.ToolCallsCount != 1 {
		t.Fatalf("expected 1 tool call, got %d", result.ToolCallsCount)
	}
}

func TestAgentLoop_MixedTools(t *testing.T) {
	mgr := NewMCPManager()
	addMockServer(t, mgr, "fs", standardMockTools(), standardCallHandler)

	registry := NewToolRegistry()
	registry.Register(&Tool{
		Name:        "local_calc",
		Description: "Calculate",
		Parameters:  []ToolParam{{Name: "expr", Type: "string", Required: true}},
		Handler: func(ctx *ToolContext, args map[string]interface{}) (interface{}, error) {
			return "42", nil
		},
	})
	mgr.InjectTools(registry)

	// LLM calls local tool first, then MCP tool, then gives final answer
	callNum := 0
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		callNum++
		switch callNum {
		case 1:
			return makeToolCallResp([]struct{ Name, Args string }{
				{"local_calc", `{"expr":"1+1"}`},
			}, ""), nil
		case 2:
			return makeToolCallResp([]struct{ Name, Args string }{
				{"mcp.fs.read_file", `{"path":"/data"}`},
			}, ""), nil
		default:
			return makeFinalResp("done"), nil
		}
	}

	loop := NewAgentLoop(llm, registry, "sys", 10, nil)
	result := loop.Run("calc and read", nil, "")

	if result.ToolCallsCount != 2 {
		t.Fatalf("expected 2 tool calls, got %d", result.ToolCallsCount)
	}
}

func TestMatchToolFilter_Wildcard(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"read_*", "read_file", true},
		{"read_*", "write_file", false},
		{"*_file", "read_file", true},
		{"*_file", "list_dir", false},
		{"list_*", "list_files", true},
		{"list_*", "list_", true},
		{"query", "query", true},
		{"query", "query2", false},
		{"*", "anything", true},
		{"db.*", "db.query", true},
		{"db.*", "db.", true},
		{"db.*", "redis.get", false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%s", tt.pattern, tt.name), func(t *testing.T) {
			got := matchToolFilter(tt.pattern, tt.name)
			if got != tt.want {
				t.Fatalf("matchToolFilter(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
			}
		})
	}
}

func TestHTTPTransport_NonOK_StatusCode(t *testing.T) {
	// We test the MCPTransportError structure directly (no real HTTP server needed)
	err := &MCPTransportError{StatusCode: 500, BodyPreview: "internal error"}
	if !err.IsRetryable() {
		t.Fatal("500 should be retryable")
	}
	err429 := &MCPTransportError{StatusCode: 429, BodyPreview: "rate limited"}
	if !err429.IsRetryable() {
		t.Fatal("429 should be retryable")
	}
	err404 := &MCPTransportError{StatusCode: 404, BodyPreview: "not found"}
	if err404.IsRetryable() {
		t.Fatal("404 should not be retryable")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error should contain status code: %s", err.Error())
	}
}
