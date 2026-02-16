package agentsdk

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// ══════════════════════════════════════════════
// StdioTransport tests
//
// Uses a real subprocess (this test binary itself with a special env var)
// to simulate an MCP server over stdio.
// ══════════════════════════════════════════════

// TestMain-like helper: when MCP_STDIO_TEST_MODE is set, this binary acts as a mock MCP server.
func init() {
	mode := os.Getenv("MCP_STDIO_TEST_MODE")
	if mode == "" {
		return
	}

	switch mode {
	case "echo":
		runStdioEchoServer()
	case "stderr_noise":
		runStdioStderrNoiseServer()
	case "crash":
		runStdioCrashServer()
	case "large_response":
		runStdioLargeResponseServer()
	}
	os.Exit(0)
}

// echo server: reads JSON-RPC from stdin, responds to initialize/tools/list/tools/call
func runStdioEchoServer() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		var req jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			resp := jsonRPCResponse{JSONRPC: "2.0", ID: 0, Error: &jsonRPCError{Code: -32700, Message: "parse error"}}
			b, _ := json.Marshal(resp)
			fmt.Println(string(b))
			continue
		}

		switch req.Method {
		case "initialize":
			result := MCPInitResult{ProtocolVersion: "2024-11-05", ServerInfo: MCPServerInfo{Name: "echo", Version: "1.0"}}
			rb, _ := json.Marshal(result)
			resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)}
			b, _ := json.Marshal(resp)
			fmt.Println(string(b))

		case "tools/list":
			tools := []MCPToolDef{
				{Name: "echo", Description: "Echo back", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"msg": map[string]interface{}{"type": "string"}}}},
			}
			wrapped := struct {
				Tools []MCPToolDef `json:"tools"`
			}{Tools: tools}
			rb, _ := json.Marshal(wrapped)
			resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)}
			b, _ := json.Marshal(resp)
			fmt.Println(string(b))

		case "tools/call":
			var params struct {
				Name      string                 `json:"name"`
				Arguments map[string]interface{} `json:"arguments"`
			}
			pb, _ := json.Marshal(req.Params)
			json.Unmarshal(pb, &params)
			msg := "echo"
			if m, ok := params.Arguments["msg"].(string); ok {
				msg = m
			}
			result := MCPToolResult{Content: []MCPContent{{Type: "text", Text: msg}}}
			rb, _ := json.Marshal(result)
			resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)}
			b, _ := json.Marshal(resp)
			fmt.Println(string(b))

		default:
			resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32601, Message: "method not found"}}
			b, _ := json.Marshal(resp)
			fmt.Println(string(b))
		}
	}
}

// stderr_noise: same as echo, but prints noise to stderr
func runStdioStderrNoiseServer() {
	fmt.Fprintln(os.Stderr, "[DEBUG] server starting up...")
	fmt.Fprintln(os.Stderr, "[WARN] some warning message")
	runStdioEchoServer()
}

// crash server: responds to initialize then exits
func runStdioCrashServer() {
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		var req jsonRPCRequest
		json.Unmarshal([]byte(scanner.Text()), &req)
		result := MCPInitResult{ProtocolVersion: "2024-11-05", ServerInfo: MCPServerInfo{Name: "crash", Version: "1.0"}}
		rb, _ := json.Marshal(result)
		resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)}
		b, _ := json.Marshal(resp)
		fmt.Println(string(b))
	}
	// Exit without reading more — simulates crash
	os.Exit(1)
}

// large_response: returns a >64K response
func runStdioLargeResponseServer() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		var req jsonRPCRequest
		json.Unmarshal([]byte(scanner.Text()), &req)

		switch req.Method {
		case "initialize":
			result := MCPInitResult{ProtocolVersion: "2024-11-05", ServerInfo: MCPServerInfo{Name: "large", Version: "1.0"}}
			rb, _ := json.Marshal(result)
			resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)}
			b, _ := json.Marshal(resp)
			fmt.Println(string(b))

		case "tools/list":
			tools := []MCPToolDef{{Name: "big", Description: "Big response", InputSchema: map[string]interface{}{"type": "object"}}}
			wrapped := struct{ Tools []MCPToolDef }{Tools: tools}
			rb, _ := json.Marshal(wrapped)
			resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)}
			b, _ := json.Marshal(resp)
			fmt.Println(string(b))

		case "tools/call":
			// Generate >100K text
			bigText := strings.Repeat("A", 100*1024)
			result := MCPToolResult{Content: []MCPContent{{Type: "text", Text: bigText}}}
			rb, _ := json.Marshal(result)
			resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: json.RawMessage(rb)}
			b, _ := json.Marshal(resp)
			fmt.Println(string(b))

		default:
			resp := jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Error: &jsonRPCError{Code: -32601, Message: "nope"}}
			b, _ := json.Marshal(resp)
			fmt.Println(string(b))
		}
	}
}

// testBinary returns the path to the current test binary.
func testBinary(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("cannot find test binary: %v", err)
	}
	return exe
}

// newStdioTestTransport creates a StdioTransport that runs this test binary in a specific mode.
func newStdioTestTransport(t *testing.T, mode string) *StdioTransport {
	t.Helper()
	// We need to run `go test -run ^$` with MCP_STDIO_TEST_MODE env to get the init() to trigger.
	// But since the test binary IS the executable, we can run it directly with -test.run=^$ to skip tests.
	exe := testBinary(t)
	return NewStdioTransport(exe, []string{"-test.run=^$"}, map[string]string{"MCP_STDIO_TEST_MODE": mode}, 10*time.Second)
}

// ── Tests ──

func TestStdioTransport_Call(t *testing.T) {
	if _, err := exec.LookPath(testBinary(t)); err != nil {
		t.Skip("cannot find test binary")
	}

	transport := newStdioTestTransport(t, "echo")
	ctx := context.Background()

	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer transport.Close()

	client := NewMCPClient(transport)

	// Initialize
	initResult, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if initResult.ServerInfo.Name != "echo" {
		t.Fatalf("expected server name=echo, got %s", initResult.ServerInfo.Name)
	}

	// ListTools
	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("unexpected tools: %+v", tools)
	}

	// CallTool
	result, err := client.CallTool(ctx, "echo", map[string]interface{}{"msg": "hello world"})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if len(result.Content) != 1 || result.Content[0].Text != "hello world" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestStdioTransport_ProcessExit(t *testing.T) {
	transport := newStdioTestTransport(t, "crash")
	ctx := context.Background()

	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer transport.Close()

	client := NewMCPClient(transport)

	// Initialize succeeds (crash server responds to first request)
	_, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Give time for process to exit
	time.Sleep(200 * time.Millisecond)

	// Next call should fail with process exited error
	_, err = client.ListTools(ctx)
	if err == nil {
		t.Fatal("expected error after process exit")
	}
	if !strings.Contains(err.Error(), "stdio") {
		t.Fatalf("expected stdio error, got: %v", err)
	}
}

func TestStdioTransport_StartupTimeout(t *testing.T) {
	// Use a non-existent command
	transport := NewStdioTransport("nonexistent_binary_that_doesnt_exist_12345", nil, nil, 5*time.Second)
	ctx := context.Background()

	err := transport.Start(ctx)
	if err == nil {
		transport.Close()
		t.Fatal("expected error for non-existent binary")
	}
	if !strings.Contains(err.Error(), "stdio start") {
		t.Fatalf("expected 'stdio start' error, got: %v", err)
	}
}

func TestStdioTransport_StderrNoise(t *testing.T) {
	transport := newStdioTestTransport(t, "stderr_noise")
	ctx := context.Background()

	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer transport.Close()

	client := NewMCPClient(transport)

	// Should work fine despite stderr output
	initResult, err := client.Initialize(ctx)
	if err != nil {
		t.Fatalf("Initialize failed despite stderr noise: %v", err)
	}
	if initResult.ServerInfo.Name != "echo" {
		t.Fatalf("unexpected server name: %s", initResult.ServerInfo.Name)
	}

	result, err := client.CallTool(ctx, "echo", map[string]interface{}{"msg": "test"})
	if err != nil {
		t.Fatalf("CallTool failed despite stderr noise: %v", err)
	}
	if result.Content[0].Text != "test" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestStdioTransport_LargeResponse(t *testing.T) {
	transport := newStdioTestTransport(t, "large_response")
	ctx := context.Background()

	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer transport.Close()

	client := NewMCPClient(transport)
	client.Initialize(ctx)

	tools, err := client.ListTools(ctx)
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	result, err := client.CallTool(ctx, "big", nil)
	if err != nil {
		t.Fatalf("CallTool failed for large response: %v", err)
	}
	if len(result.Content) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(result.Content))
	}
	// Verify >64K response was received correctly (would fail with bufio.Scanner default)
	if len(result.Content[0].Text) != 100*1024 {
		t.Fatalf("expected 100KB text, got %d bytes", len(result.Content[0].Text))
	}
}

func TestStdioTransport_CancelNoLeak(t *testing.T) {
	transport := newStdioTestTransport(t, "echo")
	ctx := context.Background()

	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer transport.Close()

	client := NewMCPClient(transport)
	client.Initialize(ctx)

	// Cancel before the call even starts — transport.Call checks ctx at entry
	cancelCtx, cancel := context.WithCancel(ctx)
	cancel()

	// This call should fail immediately with context.Canceled,
	// without writing to stdin (no residual response in the pipe).
	_, err := client.CallTool(cancelCtx, "echo", map[string]interface{}{"msg": "should not arrive"})
	if err == nil {
		t.Fatal("expected cancel error")
	}

	// The transport should still be usable after cancel
	result, err := client.CallTool(context.Background(), "echo", map[string]interface{}{"msg": "after cancel"})
	if err != nil {
		t.Fatalf("CallTool after cancel failed: %v", err)
	}
	if result.Content[0].Text != "after cancel" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
