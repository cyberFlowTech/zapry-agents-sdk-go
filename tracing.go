package agentsdk

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Tracing — Structured Span system
// ──────────────────────────────────────────────

// SpanKindType represents the type of span.
type SpanKindType string

const (
	SpanKindAgent     SpanKindType = "agent"
	SpanKindLLM       SpanKindType = "llm"
	SpanKindTool      SpanKindType = "tool"
	SpanKindGuardrail SpanKindType = "guardrail"
	SpanKindCustom    SpanKindType = "custom"
)

// TracingSpan represents a single unit of work.
type TracingSpan struct {
	SpanID     string                 `json:"span_id"`
	TraceID    string                 `json:"trace_id"`
	ParentID   string                 `json:"parent_id,omitempty"`
	Name       string                 `json:"name"`
	Kind       SpanKindType           `json:"kind"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    time.Time              `json:"end_time,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	Children   []*TracingSpan         `json:"children,omitempty"`
	Status     string                 `json:"status"` // "running", "ok", "error"
	Error      string                 `json:"error,omitempty"`
	mu         sync.Mutex
}

// DurationMs returns the span duration in milliseconds.
func (s *TracingSpan) DurationMs() float64 {
	end := s.EndTime
	if end.IsZero() {
		end = time.Now()
	}
	return float64(end.Sub(s.StartTime).Microseconds()) / 1000.0
}

// End marks the span as finished.
func (s *TracingSpan) End(status string, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.EndTime = time.Now()
	s.Status = status
	s.Error = errMsg
}

// SetAttribute sets a key-value attribute on the span.
func (s *TracingSpan) SetAttribute(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Attributes == nil {
		s.Attributes = make(map[string]interface{})
	}
	s.Attributes[key] = value
}

// AddChild adds a child span.
func (s *TracingSpan) AddChild(child *TracingSpan) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Children = append(s.Children, child)
}

// SpanExporterInterface exports finished spans.
type SpanExporterInterface interface {
	Export(span *TracingSpan)
}

// NullSpanExporter discards all spans.
type NullSpanExporter struct{}

func (e *NullSpanExporter) Export(span *TracingSpan) {}

// ConsoleSpanExporter prints spans to log.
type ConsoleSpanExporter struct{}

func (e *ConsoleSpanExporter) Export(span *TracingSpan) {
	log.Printf("[Trace] %s %s | %s | %.1fms",
		span.Kind, span.Name, span.Status, span.DurationMs())
}

// CallbackSpanExporter calls a function for each span.
type CallbackSpanExporter struct {
	Fn func(span *TracingSpan)
}

func (e *CallbackSpanExporter) Export(span *TracingSpan) {
	e.Fn(span)
}

// AgentTracer creates and manages spans.
type AgentTracer struct {
	exporter SpanExporterInterface
	enabled  bool
	traceID  string
	stack    []*TracingSpan
	mu       sync.Mutex
}

// NewAgentTracer creates a tracer.
func NewAgentTracer(exporter SpanExporterInterface, enabled bool) *AgentTracer {
	if exporter == nil {
		exporter = &NullSpanExporter{}
	}
	return &AgentTracer{exporter: exporter, enabled: enabled}
}

// NewTrace starts a new trace.
func (t *AgentTracer) NewTrace() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.traceID = randomHex(16)
	t.stack = nil
	return t.traceID
}

// StartSpan creates and starts a new span.
func (t *AgentTracer) StartSpan(name string, kind SpanKindType, attrs map[string]interface{}) *TracingSpan {
	if !t.enabled {
		return &TracingSpan{Name: name, Kind: kind, Status: "ok"}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.traceID == "" {
		t.traceID = randomHex(16)
	}

	parentID := ""
	if len(t.stack) > 0 {
		parentID = t.stack[len(t.stack)-1].SpanID
	}

	span := &TracingSpan{
		SpanID:     randomHex(6),
		TraceID:    t.traceID,
		ParentID:   parentID,
		Name:       name,
		Kind:       kind,
		StartTime:  time.Now(),
		Attributes: attrs,
		Status:     "running",
	}

	if len(t.stack) > 0 {
		t.stack[len(t.stack)-1].AddChild(span)
	}
	t.stack = append(t.stack, span)
	return span
}

// EndSpan ends the current span and exports if root.
func (t *AgentTracer) EndSpan(span *TracingSpan, status string, errMsg string) {
	if !t.enabled {
		return
	}

	span.End(status, errMsg)

	t.mu.Lock()
	defer t.mu.Unlock()

	// Pop from stack
	if len(t.stack) > 0 && t.stack[len(t.stack)-1] == span {
		t.stack = t.stack[:len(t.stack)-1]
	}

	// Export root spans
	if span.ParentID == "" {
		t.exporter.Export(span)
	}
}

// Convenience methods

func (t *AgentTracer) AgentSpan(name string) *TracingSpan {
	return t.StartSpan(name, SpanKindAgent, nil)
}

func (t *AgentTracer) LLMSpan(model string, attrs map[string]interface{}) *TracingSpan {
	name := "llm"
	if model != "" {
		name = fmt.Sprintf("llm:%s", model)
	}
	if attrs == nil {
		attrs = make(map[string]interface{})
	}
	if model != "" {
		attrs["model"] = model
	}
	return t.StartSpan(name, SpanKindLLM, attrs)
}

func (t *AgentTracer) ToolSpan(toolName string, attrs map[string]interface{}) *TracingSpan {
	return t.StartSpan(fmt.Sprintf("tool:%s", toolName), SpanKindTool, attrs)
}

func (t *AgentTracer) GuardrailSpan(name string) *TracingSpan {
	return t.StartSpan(fmt.Sprintf("guardrail:%s", name), SpanKindGuardrail, nil)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
