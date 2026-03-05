package agentsdk

import (
	"strings"
	"sync"
	"testing"
)

type captureLogger struct {
	mu      sync.Mutex
	entries []string
}

func (l *captureLogger) Debug(msg string, args ...any) { l.append("DEBUG", msg) }
func (l *captureLogger) Info(msg string, args ...any)  { l.append("INFO", msg) }
func (l *captureLogger) Warn(msg string, args ...any)  { l.append("WARN", msg) }
func (l *captureLogger) Error(msg string, args ...any) { l.append("ERROR", msg) }

func (l *captureLogger) append(level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, level+": "+msg)
}

func (l *captureLogger) contains(substr string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, item := range l.entries {
		if strings.Contains(item, substr) {
			return true
		}
	}
	return false
}

func TestSetLogger_Nil(t *testing.T) {
	if err := SetLogger(nil); err == nil {
		t.Fatal("expected error for nil logger")
	}
}

func TestSetLogger_InjectsCustomLogger(t *testing.T) {
	previous := GetLogger()
	defer func() { _ = SetLogger(previous) }()

	cl := &captureLogger{}
	if err := SetLogger(cl); err != nil {
		t.Fatalf("set logger failed: %v", err)
	}

	reg := NewToolRegistry()
	reg.Register(&Tool{Name: "demo", Handler: func(ctx *ToolContext, args map[string]interface{}) (interface{}, error) {
		return "ok", nil
	}})

	if !cl.contains("[ToolRegistry] Registered: demo") {
		t.Fatalf("expected captured registration log, got %v", cl.entries)
	}
}
