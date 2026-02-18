package agentsdk

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestLoopDetector_RepeatSameArgs(t *testing.T) {
	d := NewLoopDetector(LoopDetectorConfig{
		Enabled:        true,
		MaxRepeatCalls: 3,
		WindowSize:     10,
	})

	args := map[string]interface{}{"query": "张三"}

	// First 3 calls: check + record each
	for i := 0; i < 3; i++ {
		if w := d.Check("search", args); w != nil {
			t.Fatalf("call %d should not trigger warning yet", i+1)
		}
		d.Record("search", args)
	}

	// 4th call: 3 identical in history → triggers repeat
	w := d.Check("search", args)
	if w == nil {
		t.Fatal("expected repeat warning on 4th identical call")
	}
	if w.Type != "repeat" {
		t.Fatalf("expected type=repeat, got %s", w.Type)
	}
}

func TestLoopDetector_DifferentArgs_OK(t *testing.T) {
	d := NewLoopDetector(LoopDetectorConfig{
		Enabled:        true,
		MaxRepeatCalls: 3,
		WindowSize:     10,
	})

	// Same tool but different args each time — should not trigger
	for i := 0; i < 5; i++ {
		args := map[string]interface{}{"query": fmt.Sprintf("user_%d", i)}
		if w := d.Check("search", args); w != nil && w.Type == "repeat" {
			t.Fatalf("different args should not trigger repeat at call %d", i+1)
		}
		d.Record("search", args)
	}
}

func TestLoopDetector_SameToolFlood(t *testing.T) {
	d := NewLoopDetector(LoopDetectorConfig{
		Enabled:             true,
		MaxRepeatCalls:      10, // high to avoid repeat trigger
		MaxSameToolInWindow: 3,
		WindowSize:          5,
	})

	// 3 calls to same tool with different args
	for i := 0; i < 3; i++ {
		args := map[string]interface{}{"id": i}
		d.Check("fetch", args) // may or may not warn
		d.Record("fetch", args)
	}

	// 4th call should trigger flood
	w := d.Check("fetch", map[string]interface{}{"id": 99})
	if w == nil {
		t.Fatal("expected flood warning")
	}
	if w.Type != "flood" {
		t.Fatalf("expected type=flood, got %s", w.Type)
	}
}

func TestLoopDetector_PingPong(t *testing.T) {
	d := NewLoopDetector(LoopDetectorConfig{
		Enabled:             true,
		MaxRepeatCalls:      10,
		MaxSameToolInWindow: 10,
		WindowSize:          10,
	})

	// A, B, A → next B should trigger ping_pong
	d.Record("toolA", map[string]interface{}{"x": 1})
	d.Record("toolB", map[string]interface{}{"y": 2})
	d.Record("toolA", map[string]interface{}{"x": 3})

	w := d.Check("toolB", map[string]interface{}{"y": 4})
	if w == nil {
		t.Fatal("expected ping_pong warning")
	}
	if w.Type != "ping_pong" {
		t.Fatalf("expected type=ping_pong, got %s", w.Type)
	}
}

func TestAgentLoop_LoopDetected_StopsEarly(t *testing.T) {
	callCount := 0
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		callCount++
		// Always request the same tool call with same args
		return &LLMMessage{
			Content: "",
			ToolCalls: []ToolCallInput{
				{
					ID: fmt.Sprintf("call_%d", callCount),
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      "search",
						Arguments: `{"query":"same"}`,
					},
				},
			},
		}, nil
	}

	reg := NewToolRegistry()
	toolExecCount := 0
	reg.Register(&Tool{
		Name:       "search",
		Parameters: []ToolParam{{Name: "query", Type: "string", Required: true}},
		Handler: func(ctx *ToolContext, args map[string]interface{}) (interface{}, error) {
			toolExecCount++
			return "no results", nil
		},
	})

	loop := NewAgentLoop(llm, reg, "sys", 20, nil)
	loop.LoopDetector = NewLoopDetector(LoopDetectorConfig{
		Enabled:        true,
		MaxRepeatCalls: 3,
		WindowSize:     10,
	})

	result := loop.Run("find 张三", nil, "")

	if result.StoppedReason != "loop_detected" {
		t.Fatalf("expected loop_detected, got %s", result.StoppedReason)
	}
	// Should stop after MaxRepeatCalls (3 executions, 4th check triggers)
	if toolExecCount > 3 {
		t.Fatalf("expected <= 3 tool executions, got %d", toolExecCount)
	}

	_ = json.Marshal // keep import
}
