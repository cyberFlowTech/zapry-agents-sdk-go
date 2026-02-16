package agentsdk

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// MCP Client — Transport layer
// ──────────────────────────────────────────────

const mcpMaxErrorBodySize = 128 * 1024 // 128KB limit for error response bodies

// MCPTransport is the low-level transport interface using request-response semantics.
type MCPTransport interface {
	// Start initializes the transport (HTTP: no-op, Stdio: starts process).
	Start(ctx context.Context) error
	// Call sends a request payload and waits for the response (single RPC).
	Call(ctx context.Context, payload []byte) ([]byte, error)
	// Close shuts down the transport.
	Close() error
}

// MCPTransportError wraps HTTP non-2xx responses with status code and body preview.
type MCPTransportError struct {
	StatusCode  int
	BodyPreview string // truncated to 512 chars
}

func (e *MCPTransportError) Error() string {
	return fmt.Sprintf("mcp: http %d: %s", e.StatusCode, e.BodyPreview)
}

// IsRetryable returns true for 5xx and 429 status codes.
func (e *MCPTransportError) IsRetryable() bool {
	return e.StatusCode >= 500 || e.StatusCode == 429
}

// ──────────────────────────────────────────────
// HTTPTransport
// ──────────────────────────────────────────────

// HTTPTransport implements MCPTransport over HTTP POST.
type HTTPTransport struct {
	url     string
	headers map[string]string
	timeout time.Duration
	client  *http.Client
}

// NewHTTPTransport creates an HTTP transport for the given endpoint.
func NewHTTPTransport(url string, headers map[string]string, timeout time.Duration) *HTTPTransport {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &HTTPTransport{
		url:     url,
		headers: headers,
		timeout: timeout,
		client:  &http.Client{Timeout: timeout},
	}
}

func (t *HTTPTransport) Start(ctx context.Context) error { return nil }

func (t *HTTPTransport) Call(ctx context.Context, payload []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", t.url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("mcp: http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp: http call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		limited := io.LimitReader(resp.Body, mcpMaxErrorBodySize)
		body, _ := io.ReadAll(limited)
		preview := string(body)
		if len(preview) > 512 {
			preview = preview[:512] + "..."
		}
		return nil, &MCPTransportError{
			StatusCode:  resp.StatusCode,
			BodyPreview: preview,
		}
	}

	return io.ReadAll(resp.Body)
}

func (t *HTTPTransport) Close() error { return nil }

// ──────────────────────────────────────────────
// InProcessTransport (for testing)
// ──────────────────────────────────────────────

// InProcessTransport implements MCPTransport by calling a handler function directly.
// Used for deterministic testing without external processes or network.
type InProcessTransport struct {
	handler func(request []byte) ([]byte, error)
}

// NewInProcessTransport creates a transport that delegates to the given handler.
func NewInProcessTransport(handler func([]byte) ([]byte, error)) *InProcessTransport {
	return &InProcessTransport{handler: handler}
}

func (t *InProcessTransport) Start(ctx context.Context) error { return nil }

func (t *InProcessTransport) Call(ctx context.Context, payload []byte) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	type result struct {
		data []byte
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		data, err := t.handler(payload)
		ch <- result{data, err}
	}()

	select {
	case r := <-ch:
		return r.data, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *InProcessTransport) Close() error { return nil }

// ──────────────────────────────────────────────
// StdioTransport
// ──────────────────────────────────────────────

// StdioTransport implements MCPTransport by launching a child process
// and communicating via stdin/stdout using newline-delimited JSON.
//
// Architecture: a single long-lived reader goroutine reads stdout lines
// into a channel. Call() writes to stdin and reads from the channel.
// This avoids goroutine leaks on cancel.
//
// stderr is consumed by a separate goroutine and logged (never parsed as JSON).
type StdioTransport struct {
	command string
	args    []string
	env     map[string]string
	timeout time.Duration

	cmd    *exec.Cmd
	stdin  io.WriteCloser
	lines  chan []byte  // reader goroutine writes lines here
	errc   chan error   // reader goroutine error
	done   chan struct{} // closed when process exits
	mu     sync.Mutex   // serializes writes to stdin
}

// NewStdioTransport creates a stdio transport for the given command.
func NewStdioTransport(command string, args []string, env map[string]string, timeout time.Duration) *StdioTransport {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &StdioTransport{
		command: command,
		args:    args,
		env:     env,
		timeout: timeout,
	}
}

// Start launches the child process and starts reader goroutines.
func (t *StdioTransport) Start(ctx context.Context) error {
	t.cmd = exec.CommandContext(ctx, t.command, t.args...)

	// Set environment
	if len(t.env) > 0 {
		t.cmd.Env = os.Environ()
		for k, v := range t.env {
			t.cmd.Env = append(t.cmd.Env, k+"="+v)
		}
	}

	var err error
	t.stdin, err = t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("mcp: stdio stdin pipe: %w", err)
	}

	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("mcp: stdio stdout pipe: %w", err)
	}

	stderr, err := t.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("mcp: stdio stderr pipe: %w", err)
	}

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("mcp: stdio start %q: %w", t.command, err)
	}

	t.lines = make(chan []byte, 16)
	t.errc = make(chan error, 1)
	t.done = make(chan struct{})

	// Long-lived reader goroutine: reads stdout lines into channel.
	// Uses bufio.NewReaderSize with 1MB buffer to avoid 64K Scanner limit.
	go func() {
		reader := bufio.NewReaderSize(stdout, 1024*1024)
		for {
			line, err := reader.ReadBytes('\n')
			if err != nil {
				if len(line) > 0 {
					t.lines <- bytes.TrimSpace(line)
				}
				t.errc <- err
				return
			}
			trimmed := bytes.TrimSpace(line)
			if len(trimmed) > 0 {
				t.lines <- trimmed
			}
		}
	}()

	// stderr consumer: log only, never parsed as JSON.
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("[MCP:stdio:%s] stderr: %s", t.command, scanner.Text())
		}
	}()

	// Process exit watcher.
	go func() {
		t.cmd.Wait()
		close(t.done)
	}()

	return nil
}

// Call sends a JSON-RPC request to stdin and reads one response line from stdout.
func (t *StdioTransport) Call(ctx context.Context, payload []byte) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check cancellation before writing
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Check if process is still alive
	select {
	case <-t.done:
		return nil, fmt.Errorf("mcp: stdio process exited")
	default:
	}

	// Write request (newline-delimited)
	if _, err := t.stdin.Write(append(payload, '\n')); err != nil {
		return nil, fmt.Errorf("mcp: stdio write: %w", err)
	}

	// Read one response line from the channel
	select {
	case line := <-t.lines:
		return line, nil
	case err := <-t.errc:
		return nil, fmt.Errorf("mcp: stdio read: %w", err)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.done:
		return nil, fmt.Errorf("mcp: stdio process exited during read")
	}
}

// Close shuts down the child process gracefully.
func (t *StdioTransport) Close() error {
	if t.stdin != nil {
		t.stdin.Close()
	}

	if t.cmd == nil || t.cmd.Process == nil {
		return nil
	}

	// Wait for process exit with timeout
	select {
	case <-t.done:
		return nil
	case <-time.After(5 * time.Second):
		t.cmd.Process.Kill()
		<-t.done
		return nil
	}
}
