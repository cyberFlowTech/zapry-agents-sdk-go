package agentsdk

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// ══════════════════════════════════════════════
// Test helpers
// ══════════════════════════════════════════════

func makeFinalResp(content string) *LLMMessage {
	return &LLMMessage{Content: content}
}

func makeToolCallResp(calls []struct{ Name, Args string }, content string) *LLMMessage {
	var tcs []ToolCallInput
	for i, c := range calls {
		tc := ToolCallInput{ID: fmt.Sprintf("call_%d", i)}
		tc.Function.Name = c.Name
		tc.Function.Arguments = c.Args
		tcs = append(tcs, tc)
	}
	return &LLMMessage{Content: content, ToolCalls: tcs}
}

func testRegistry() *ToolRegistry {
	r := NewToolRegistry()
	r.Register(&Tool{
		Name:       "get_weather",
		Parameters: []ToolParam{{Name: "city", Type: "string", Required: true}},
		Handler: func(ctx *ToolContext, args map[string]interface{}) (interface{}, error) {
			return fmt.Sprintf("%s: 25°C", args["city"]), nil
		},
	})
	r.Register(&Tool{
		Name: "add",
		Parameters: []ToolParam{
			{Name: "a", Type: "integer", Required: true},
			{Name: "b", Type: "integer", Required: true},
		},
		Handler: func(ctx *ToolContext, args map[string]interface{}) (interface{}, error) {
			a, _ := args["a"].(float64)
			b, _ := args["b"].(float64)
			return int(a + b), nil
		},
	})
	r.Register(&Tool{
		Name:       "search",
		Parameters: []ToolParam{{Name: "query", Type: "string", Required: true}},
		Handler: func(ctx *ToolContext, args map[string]interface{}) (interface{}, error) {
			return fmt.Sprintf("Results for: %s", args["query"]), nil
		},
	})
	return r
}

// ══════════════════════════════════════════════
// Core behavior tests
// ══════════════════════════════════════════════

func TestAgentLoop_DirectAnswer(t *testing.T) {
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		return makeFinalResp("Hello!"), nil
	}
	loop := NewAgentLoop(llm, testRegistry(), "sys", 10, nil)
	result := loop.Run("hi", nil, "")

	if result.FinalOutput != "Hello!" {
		t.Fatalf("expected Hello!, got %s", result.FinalOutput)
	}
	if result.TotalTurns != 1 {
		t.Fatalf("expected 1 turn, got %d", result.TotalTurns)
	}
	if result.ToolCallsCount != 0 {
		t.Fatal("expected 0 tool calls")
	}
	if result.StoppedReason != "completed" {
		t.Fatalf("expected completed, got %s", result.StoppedReason)
	}
}

func TestAgentLoop_SingleToolCall(t *testing.T) {
	callCount := 0
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		callCount++
		if callCount == 1 {
			return makeToolCallResp([]struct{ Name, Args string }{
				{"get_weather", `{"city":"Shanghai"}`},
			}, ""), nil
		}
		return makeFinalResp("Shanghai is 25°C."), nil
	}

	loop := NewAgentLoop(llm, testRegistry(), "", 10, nil)
	result := loop.Run("weather?", nil, "")

	if result.FinalOutput != "Shanghai is 25°C." {
		t.Fatalf("unexpected output: %s", result.FinalOutput)
	}
	if result.TotalTurns != 2 {
		t.Fatalf("expected 2 turns, got %d", result.TotalTurns)
	}
	if result.ToolCallsCount != 1 {
		t.Fatalf("expected 1 tool call, got %d", result.ToolCallsCount)
	}
	if result.Turns[0].ToolCalls[0].ToolName != "get_weather" {
		t.Fatal("expected get_weather tool call")
	}
}

func TestAgentLoop_MultipleToolCallsSingleTurn(t *testing.T) {
	callCount := 0
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		callCount++
		if callCount == 1 {
			return makeToolCallResp([]struct{ Name, Args string }{
				{"get_weather", `{"city":"Shanghai"}`},
				{"get_weather", `{"city":"Beijing"}`},
			}, ""), nil
		}
		return makeFinalResp("Both checked."), nil
	}

	loop := NewAgentLoop(llm, testRegistry(), "", 10, nil)
	result := loop.Run("both cities", nil, "")

	if result.ToolCallsCount != 2 {
		t.Fatalf("expected 2 tool calls, got %d", result.ToolCallsCount)
	}
	if len(result.Turns[0].ToolCalls) != 2 {
		t.Fatal("expected 2 calls in first turn")
	}
}

func TestAgentLoop_MultiTurnToolCalls(t *testing.T) {
	callCount := 0
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		callCount++
		if callCount == 1 {
			return makeToolCallResp([]struct{ Name, Args string }{
				{"search", `{"query":"restaurants"}`},
			}, ""), nil
		}
		if callCount == 2 {
			return makeToolCallResp([]struct{ Name, Args string }{
				{"get_weather", `{"city":"Shanghai"}`},
			}, ""), nil
		}
		return makeFinalResp("Found restaurants, weather is 25°C."), nil
	}

	loop := NewAgentLoop(llm, testRegistry(), "", 10, nil)
	result := loop.Run("find restaurants + weather", nil, "")

	if result.TotalTurns != 3 {
		t.Fatalf("expected 3 turns, got %d", result.TotalTurns)
	}
	if result.ToolCallsCount != 2 {
		t.Fatalf("expected 2 total tool calls, got %d", result.ToolCallsCount)
	}
}

// ══════════════════════════════════════════════
// Max turns
// ══════════════════════════════════════════════

func TestAgentLoop_MaxTurnsReached(t *testing.T) {
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		return makeToolCallResp([]struct{ Name, Args string }{
			{"search", `{"query":"infinite"}`},
		}, "still thinking"), nil
	}

	loop := NewAgentLoop(llm, testRegistry(), "", 3, nil)
	result := loop.Run("loop forever", nil, "")

	if result.StoppedReason != "max_turns" {
		t.Fatalf("expected max_turns, got %s", result.StoppedReason)
	}
	if result.TotalTurns != 3 {
		t.Fatalf("expected 3 turns, got %d", result.TotalTurns)
	}
}

func TestAgentLoop_MaxTurns1(t *testing.T) {
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		return makeToolCallResp([]struct{ Name, Args string }{
			{"search", `{"query":"test"}`},
		}, ""), nil
	}

	loop := NewAgentLoop(llm, testRegistry(), "", 1, nil)
	result := loop.Run("test", nil, "")

	if result.TotalTurns != 1 {
		t.Fatalf("expected 1 turn, got %d", result.TotalTurns)
	}
}

// ══════════════════════════════════════════════
// Error handling
// ══════════════════════════════════════════════

func TestAgentLoop_ToolError(t *testing.T) {
	callCount := 0
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		callCount++
		if callCount == 1 {
			return makeToolCallResp([]struct{ Name, Args string }{
				{"nonexistent", `{}`},
			}, ""), nil
		}
		return makeFinalResp("Tool not available."), nil
	}

	loop := NewAgentLoop(llm, testRegistry(), "", 10, nil)
	result := loop.Run("test", nil, "")

	if result.StoppedReason != "completed" {
		t.Fatalf("expected completed, got %s", result.StoppedReason)
	}
	if result.Turns[0].ToolCalls[0].Error == "" {
		t.Fatal("expected tool error")
	}
}

func TestAgentLoop_LLMError(t *testing.T) {
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		return nil, fmt.Errorf("API connection failed")
	}

	loop := NewAgentLoop(llm, testRegistry(), "", 10, nil)
	result := loop.Run("test", nil, "")

	if result.StoppedReason != "error" {
		t.Fatalf("expected error, got %s", result.StoppedReason)
	}
}

// ══════════════════════════════════════════════
// Hooks
// ══════════════════════════════════════════════

func TestAgentLoop_HooksCalled(t *testing.T) {
	var events []string

	hooks := &AgentLoopHooks{
		OnLLMStart:  func(turn int, msgs []map[string]interface{}) { events = append(events, fmt.Sprintf("llm_start:%d", turn)) },
		OnLLMEnd:    func(turn int, resp *LLMMessage) { events = append(events, fmt.Sprintf("llm_end:%d", turn)) },
		OnToolStart: func(name string, args map[string]interface{}) { events = append(events, "tool_start:"+name) },
		OnToolEnd:   func(name, result, err string) { events = append(events, "tool_end:"+name) },
		OnTurnEnd:   func(turn *TurnRecord) { events = append(events, fmt.Sprintf("turn_end:%d", turn.TurnNumber)) },
	}

	callCount := 0
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		callCount++
		if callCount == 1 {
			return makeToolCallResp([]struct{ Name, Args string }{
				{"get_weather", `{"city":"SH"}`},
			}, ""), nil
		}
		return makeFinalResp("Done"), nil
	}

	loop := NewAgentLoop(llm, testRegistry(), "", 10, hooks)
	loop.Run("test", nil, "")

	has := func(s string) bool {
		for _, e := range events {
			if e == s {
				return true
			}
		}
		return false
	}

	if !has("llm_start:1") || !has("llm_end:1") || !has("tool_start:get_weather") || !has("tool_end:get_weather") || !has("turn_end:1") {
		t.Fatalf("missing expected events: %v", events)
	}
}

func TestAgentLoop_ErrorHook(t *testing.T) {
	var errors []string
	hooks := &AgentLoopHooks{
		OnError: func(err error) { errors = append(errors, err.Error()) },
	}

	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		return nil, fmt.Errorf("boom")
	}

	loop := NewAgentLoop(llm, testRegistry(), "", 10, hooks)
	loop.Run("test", nil, "")

	if len(errors) != 1 || errors[0] != "boom" {
		t.Fatalf("expected error hook with 'boom', got %v", errors)
	}
}

// ══════════════════════════════════════════════
// Messages
// ══════════════════════════════════════════════

func TestAgentLoop_SystemPromptInMessages(t *testing.T) {
	var captured []map[string]interface{}
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		captured = msgs
		return makeFinalResp("ok"), nil
	}

	loop := NewAgentLoop(llm, testRegistry(), "You are helpful.", 10, nil)
	loop.Run("hi", nil, "")

	if captured[0]["role"] != "system" || captured[0]["content"] != "You are helpful." {
		t.Fatal("system prompt not found")
	}
}

func TestAgentLoop_ExtraContext(t *testing.T) {
	var captured []map[string]interface{}
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		captured = msgs
		return makeFinalResp("ok"), nil
	}

	loop := NewAgentLoop(llm, testRegistry(), "sys", 10, nil)
	loop.Run("hi", nil, "User is 25 years old")

	found := false
	for _, m := range captured {
		if c, ok := m["content"].(string); ok && c == "User is 25 years old" {
			found = true
		}
	}
	if !found {
		t.Fatal("extra context not found in messages")
	}
}

func TestAgentLoop_ResultMessagesComplete(t *testing.T) {
	callCount := 0
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		callCount++
		if callCount == 1 {
			return makeToolCallResp([]struct{ Name, Args string }{
				{"add", `{"a":1,"b":2}`},
			}, ""), nil
		}
		return makeFinalResp("3"), nil
	}

	loop := NewAgentLoop(llm, testRegistry(), "", 10, nil)
	result := loop.Run("1+2", nil, "")

	roles := map[string]bool{}
	for _, m := range result.Messages {
		if r, ok := m["role"].(string); ok {
			roles[r] = true
		}
	}
	if !roles["user"] || !roles["assistant"] || !roles["tool"] {
		t.Fatalf("expected user/assistant/tool roles, got %v", roles)
	}
}

// ══════════════════════════════════════════════
// Edge cases
// ══════════════════════════════════════════════

func TestAgentLoop_EmptyRegistry(t *testing.T) {
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		if tools != nil {
			t.Fatal("expected nil tools for empty registry")
		}
		return makeFinalResp("No tools."), nil
	}

	loop := NewAgentLoop(llm, NewToolRegistry(), "", 10, nil)
	result := loop.Run("test", nil, "")

	if result.FinalOutput != "No tools." {
		t.Fatalf("unexpected: %s", result.FinalOutput)
	}
}

func TestAgentLoop_ToolReturnsNonString(t *testing.T) {
	callCount := 0
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		callCount++
		if callCount == 1 {
			return makeToolCallResp([]struct{ Name, Args string }{
				{"add", `{"a":3,"b":4}`},
			}, ""), nil
		}
		return makeFinalResp("7"), nil
	}

	loop := NewAgentLoop(llm, testRegistry(), "", 10, nil)
	result := loop.Run("3+4", nil, "")

	if result.Turns[0].ToolCalls[0].Result != "7" {
		t.Fatalf("expected '7', got %s", result.Turns[0].ToolCalls[0].Result)
	}
}

func TestAgentLoop_ConversationHistory(t *testing.T) {
	var captured []map[string]interface{}
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		captured = msgs
		return makeFinalResp("ok"), nil
	}

	history := []map[string]interface{}{
		{"role": "user", "content": "previous"},
		{"role": "assistant", "content": "prev answer"},
	}
	loop := NewAgentLoop(llm, testRegistry(), "", 10, nil)
	loop.Run("new", history, "")

	found := false
	for _, m := range captured {
		if m["content"] == "previous" {
			found = true
		}
	}
	if !found {
		t.Fatal("conversation history not in messages")
	}
}

func TestAgentLoop_JSONSerialization(t *testing.T) {
	callCount := 0
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		callCount++
		if callCount == 1 {
			return makeToolCallResp([]struct{ Name, Args string }{
				{"get_weather", `{"city":"SH"}`},
			}, ""), nil
		}
		return makeFinalResp("Done"), nil
	}

	loop := NewAgentLoop(llm, testRegistry(), "sys", 10, nil)
	result := loop.Run("test", nil, "")

	// Verify result is JSON-serializable
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty JSON")
	}
}

// ══════════════════════════════════════════════
// RunContext / Cancel tests
// ══════════════════════════════════════════════

func TestRunContext_Cancelled_BeforeLLM(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		t.Fatal("LLM should not be called when ctx is already cancelled")
		return nil, nil
	}

	loop := NewAgentLoop(llm, testRegistry(), "sys", 10, nil)
	result := loop.RunContext(ctx, "hello", nil, "")

	if result.StoppedReason != "cancelled" {
		t.Fatalf("expected stopped_reason=cancelled, got %s", result.StoppedReason)
	}
	if result.TotalTurns != 0 {
		t.Fatalf("expected 0 turns, got %d", result.TotalTurns)
	}
}

func TestRunContext_Cancelled_DuringToolExec(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	toolExecCount := 0
	callCount := 0
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		callCount++
		if callCount == 1 {
			// Request two tool calls; cancel after the first executes
			return makeToolCallResp([]struct{ Name, Args string }{
				{"get_weather", `{"city":"A"}`},
				{"get_weather", `{"city":"B"}`},
			}, ""), nil
		}
		return makeFinalResp("done"), nil
	}

	reg := NewToolRegistry()
	reg.Register(&Tool{
		Name:       "get_weather",
		Parameters: []ToolParam{{Name: "city", Type: "string", Required: true}},
		Handler: func(tc *ToolContext, args map[string]interface{}) (interface{}, error) {
			toolExecCount++
			if toolExecCount == 1 {
				cancel() // cancel after first tool
			}
			return fmt.Sprintf("%s: 25°C", args["city"]), nil
		},
	})

	loop := NewAgentLoop(llm, reg, "sys", 10, nil)
	result := loop.RunContext(ctx, "weather", nil, "")

	if result.StoppedReason != "cancelled" {
		t.Fatalf("expected cancelled, got %s", result.StoppedReason)
	}
	// First tool should have executed, second should have been skipped
	if toolExecCount != 1 {
		t.Fatalf("expected 1 tool execution, got %d", toolExecCount)
	}
}

func TestRunContext_WithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Use LLMFnCtx which respects context cancellation
	llmCtx := func(ctx context.Context, msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
			return makeFinalResp("done"), nil
		}
	}

	loop := NewAgentLoop(nil, testRegistry(), "sys", 10, nil)
	loop.LLMFnCtx = llmCtx
	result := loop.RunContext(ctx, "hello", nil, "")

	if result.StoppedReason != "cancelled" {
		t.Fatalf("expected cancelled, got %s", result.StoppedReason)
	}
}

func TestRunContext_BackwardsCompat(t *testing.T) {
	// Run() without context should behave exactly as before
	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		return makeFinalResp("Hello!"), nil
	}

	loop := NewAgentLoop(llm, testRegistry(), "sys", 10, nil)
	result := loop.Run("hi", nil, "")

	if result.FinalOutput != "Hello!" {
		t.Fatalf("expected Hello!, got %s", result.FinalOutput)
	}
	if result.StoppedReason != "completed" {
		t.Fatalf("expected completed, got %s", result.StoppedReason)
	}
}
