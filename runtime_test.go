package agentsdk

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type mockShutdowner struct {
	called bool
	err    error
}

func (m *mockShutdowner) Shutdown(ctx context.Context) error {
	m.called = true
	return m.err
}

type mockStopper struct {
	called bool
}

func (m *mockStopper) Stop() {
	m.called = true
}

type mockDisconnecter struct {
	called bool
	err    error
}

func (m *mockDisconnecter) DisconnectAll() error {
	m.called = true
	return m.err
}

type mockCloser struct {
	called bool
	err    error
}

func (m *mockCloser) Close() error {
	m.called = true
	return m.err
}

func TestSDKRuntime_Shutdown_CallsAllComponents(t *testing.T) {
	auto := &mockShutdowner{}
	scheduler := &mockStopper{}
	mcp := &mockDisconnecter{}
	closer := &mockCloser{}

	rt := &SDKRuntime{
		AutoConversation:   auto,
		ProactiveScheduler: scheduler,
		MCPManager:         mcp,
		Closers:            []io.Closer{closer},
	}

	if err := rt.Shutdown(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !auto.called {
		t.Fatal("auto conversation shutdown should be called")
	}
	if !scheduler.called {
		t.Fatal("scheduler stop should be called")
	}
	if !mcp.called {
		t.Fatal("mcp disconnect should be called")
	}
	if !closer.called {
		t.Fatal("closer should be called")
	}
}

func TestSDKRuntime_Shutdown_CollectsErrors(t *testing.T) {
	rt := &SDKRuntime{
		AutoConversation: &mockShutdowner{err: errors.New("auto err")},
		MCPManager:       &mockDisconnecter{err: errors.New("mcp err")},
		ShutdownHooks: []ShutdownFunc{
			func(ctx context.Context) error { return errors.New("hook err") },
		},
	}

	err := rt.Shutdown(context.Background())
	if err == nil {
		t.Fatal("expected aggregated error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "auto err") || !strings.Contains(msg, "mcp err") || !strings.Contains(msg, "hook err") {
		t.Fatalf("unexpected error aggregation: %v", err)
	}
}
