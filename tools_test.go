package agentsdk

import (
	"encoding/json"
	"fmt"
	"testing"
)

// ══════════════════════════════════════════════
// Tool schema tests
// ══════════════════════════════════════════════

func TestTool_ToJSONSchema(t *testing.T) {
	tool := &Tool{
		Name:        "test",
		Description: "Test tool",
		Parameters: []ToolParam{
			{Name: "x", Type: "string", Description: "param x", Required: true},
			{Name: "y", Type: "integer", Required: false, Default: 5},
		},
	}
	schema := tool.ToJSONSchema()
	if schema["name"] != "test" {
		t.Fatal("name mismatch")
	}
	params := schema["parameters"].(map[string]interface{})
	props := params["properties"].(map[string]interface{})
	if _, ok := props["x"]; !ok {
		t.Fatal("missing property x")
	}
	req := params["required"].([]string)
	if len(req) != 1 || req[0] != "x" {
		t.Fatalf("expected required=[x], got %v", req)
	}
}

func TestTool_ToOpenAISchema(t *testing.T) {
	tool := &Tool{Name: "t", Description: "d"}
	schema := tool.ToOpenAISchema()
	if schema["type"] != "function" {
		t.Fatal("expected type=function")
	}
	fn := schema["function"].(map[string]interface{})
	if fn["name"] != "t" {
		t.Fatal("expected name=t")
	}
}

// ══════════════════════════════════════════════
// ToolRegistry tests
// ══════════════════════════════════════════════

func makeTestTool(name string, handler ToolHandlerFunc) *Tool {
	return &Tool{
		Name:        name,
		Description: name + " tool",
		Parameters: []ToolParam{
			{Name: "x", Type: "string", Required: true},
		},
		Handler: handler,
	}
}

func TestToolRegistry_RegisterGet(t *testing.T) {
	r := NewToolRegistry()
	r.Register(makeTestTool("hello", nil))
	if !r.Contains("hello") {
		t.Fatal("should contain hello")
	}
	if r.Get("hello") == nil {
		t.Fatal("Get should return tool")
	}
	if r.Get("nonexistent") != nil {
		t.Fatal("Get should return nil for missing")
	}
}

func TestToolRegistry_ListNames(t *testing.T) {
	r := NewToolRegistry()
	r.Register(makeTestTool("a", nil))
	r.Register(makeTestTool("b", nil))
	if r.Len() != 2 {
		t.Fatalf("expected 2, got %d", r.Len())
	}
	names := r.Names()
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found["a"] || !found["b"] {
		t.Fatalf("expected a and b, got %v", names)
	}
}

func TestToolRegistry_Remove(t *testing.T) {
	r := NewToolRegistry()
	r.Register(makeTestTool("rm", nil))
	r.Remove("rm")
	if r.Contains("rm") {
		t.Fatal("should not contain rm after remove")
	}
}

func TestToolRegistry_ToJSONSchema(t *testing.T) {
	r := NewToolRegistry()
	r.Register(makeTestTool("t1", nil))
	schemas := r.ToJSONSchema()
	if len(schemas) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(schemas))
	}
}

func TestToolRegistry_ToOpenAISchema(t *testing.T) {
	r := NewToolRegistry()
	r.Register(makeTestTool("t1", nil))
	schemas := r.ToOpenAISchema()
	if schemas[0]["type"] != "function" {
		t.Fatal("expected type=function")
	}
}

func TestToolRegistry_Execute(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&Tool{
		Name:        "add",
		Description: "Add",
		Parameters: []ToolParam{
			{Name: "a", Type: "integer", Required: true},
			{Name: "b", Type: "integer", Required: true},
		},
		Handler: func(ctx *ToolContext, args map[string]interface{}) (interface{}, error) {
			a := int(args["a"].(float64))
			b := int(args["b"].(float64))
			return a + b, nil
		},
	})

	result, err := r.Execute("add", map[string]interface{}{"a": 3.0, "b": 5.0}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != 8 {
		t.Fatalf("expected 8, got %v", result)
	}
}

func TestToolRegistry_ExecuteWithDefaults(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&Tool{
		Name: "greet",
		Parameters: []ToolParam{
			{Name: "name", Type: "string", Required: true},
			{Name: "greeting", Type: "string", Required: false, Default: "Hello"},
		},
		Handler: func(ctx *ToolContext, args map[string]interface{}) (interface{}, error) {
			return fmt.Sprintf("%s %s", args["greeting"], args["name"]), nil
		},
	})

	result, err := r.Execute("greet", map[string]interface{}{"name": "World"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "Hello World" {
		t.Fatalf("expected 'Hello World', got %v", result)
	}
}

func TestToolRegistry_ExecuteMissingRequired(t *testing.T) {
	r := NewToolRegistry()
	r.Register(makeTestTool("t", func(ctx *ToolContext, args map[string]interface{}) (interface{}, error) {
		return nil, nil
	}))

	_, err := r.Execute("t", map[string]interface{}{}, nil)
	if err == nil {
		t.Fatal("expected error for missing required arg")
	}
}

func TestToolRegistry_ExecuteUnknown(t *testing.T) {
	r := NewToolRegistry()
	_, err := r.Execute("nonexistent", nil, nil)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}

func TestToolRegistry_ExecuteWithContext(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&Tool{
		Name:       "ctx_tool",
		Parameters: []ToolParam{{Name: "msg", Type: "string", Required: true}},
		Handler: func(ctx *ToolContext, args map[string]interface{}) (interface{}, error) {
			return fmt.Sprintf("%s: %s", ctx.ToolName, args["msg"]), nil
		},
	})

	result, err := r.Execute("ctx_tool", map[string]interface{}{"msg": "hi"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "ctx_tool: hi" {
		t.Fatalf("expected 'ctx_tool: hi', got %v", result)
	}
}

func TestToolRegistry_ExecuteWithCustomContext(t *testing.T) {
	r := NewToolRegistry()
	r.Register(&Tool{
		Name: "extra_tool",
		Handler: func(ctx *ToolContext, args map[string]interface{}) (interface{}, error) {
			return ctx.Extra["key"], nil
		},
	})

	ctx := &ToolContext{Extra: map[string]interface{}{"key": "value"}}
	result, err := r.Execute("extra_tool", nil, ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "value" {
		t.Fatalf("expected 'value', got %v", result)
	}
}

// ══════════════════════════════════════════════
// OpenAIToolAdapter tests
// ══════════════════════════════════════════════

func newTestAdapter() *OpenAIToolAdapter {
	r := NewToolRegistry()
	r.Register(&Tool{
		Name:        "get_weather",
		Description: "Get weather",
		Parameters:  []ToolParam{{Name: "city", Type: "string", Required: true}},
		Handler: func(ctx *ToolContext, args map[string]interface{}) (interface{}, error) {
			return fmt.Sprintf("%s: 25°C", args["city"]), nil
		},
	})
	r.Register(&Tool{
		Name:        "add",
		Description: "Add",
		Parameters: []ToolParam{
			{Name: "a", Type: "integer", Required: true},
			{Name: "b", Type: "integer", Required: true},
		},
		Handler: func(ctx *ToolContext, args map[string]interface{}) (interface{}, error) {
			a := int(args["a"].(float64))
			b := int(args["b"].(float64))
			return a + b, nil
		},
	})
	return NewOpenAIToolAdapter(r)
}

func TestOpenAIToolAdapter_ToOpenAITools(t *testing.T) {
	adapter := newTestAdapter()
	tools := adapter.ToOpenAITools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	for _, tool := range tools {
		if tool["type"] != "function" {
			t.Fatal("expected type=function")
		}
	}
}

func TestOpenAIToolAdapter_HandleToolCalls(t *testing.T) {
	adapter := newTestAdapter()
	calls := []ToolCallInput{
		{
			ID: "call_1",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: "get_weather", Arguments: `{"city": "Shanghai"}`},
		},
	}
	results := adapter.HandleToolCalls(calls)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ToolCallID != "call_1" {
		t.Fatal("wrong call ID")
	}
	if results[0].Error != "" {
		t.Fatalf("unexpected error: %s", results[0].Error)
	}
	if results[0].Content != "Shanghai: 25°C" {
		t.Fatalf("expected 'Shanghai: 25°C', got %s", results[0].Content)
	}
}

func TestOpenAIToolAdapter_HandleMultipleCalls(t *testing.T) {
	adapter := newTestAdapter()
	calls := []ToolCallInput{
		{
			ID: "c1",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: "get_weather", Arguments: `{"city": "Beijing"}`},
		},
		{
			ID: "c2",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: "add", Arguments: `{"a": 1, "b": 2}`},
		},
	}
	results := adapter.HandleToolCalls(calls)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[1].Content != "3" {
		t.Fatalf("expected '3', got %s", results[1].Content)
	}
}

func TestOpenAIToolAdapter_HandleUnknownTool(t *testing.T) {
	adapter := newTestAdapter()
	calls := []ToolCallInput{
		{
			ID: "c1",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: "unknown", Arguments: "{}"},
		},
	}
	results := adapter.HandleToolCalls(calls)
	if results[0].Error == "" {
		t.Fatal("expected error for unknown tool")
	}
}

func TestOpenAIToolAdapter_ResultsToMessages(t *testing.T) {
	adapter := newTestAdapter()
	results := []ToolCallResult{
		{ToolCallID: "c1", Name: "t", Content: "ok"},
		{ToolCallID: "c2", Name: "t", Error: "fail"},
	}
	msgs := adapter.ResultsToMessages(results)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0]["role"] != "tool" {
		t.Fatal("expected role=tool")
	}
	if msgs[0]["content"] != "ok" {
		t.Fatal("expected content=ok")
	}
	if msgs[1]["content"] != "fail" {
		t.Fatal("expected error content")
	}
}

func TestToolCallResult_ToMessage(t *testing.T) {
	r := ToolCallResult{ToolCallID: "c1", Name: "t", Content: "data"}
	m := r.ToMessage()
	if m["role"] != "tool" || m["tool_call_id"] != "c1" || m["content"] != "data" {
		t.Fatalf("unexpected message: %v", m)
	}
}

func TestToolCallResult_ErrorToMessage(t *testing.T) {
	r := ToolCallResult{ToolCallID: "c1", Name: "t", Error: "oops"}
	m := r.ToMessage()
	if m["content"] != "oops" {
		t.Fatalf("expected error in content, got %s", m["content"])
	}
}

// Verify JSON serialization roundtrip for ToolCallInput
func TestToolCallInput_JSON(t *testing.T) {
	input := ToolCallInput{ID: "c1"}
	input.Function.Name = "test"
	input.Function.Arguments = `{"x": 1}`

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}

	var parsed ToolCallInput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.ID != "c1" || parsed.Function.Name != "test" {
		t.Fatalf("roundtrip failed: %+v", parsed)
	}
}

// ══════════════════════════════════════════════
// RawJSONSchema tests
// ══════════════════════════════════════════════

func TestTool_RawJSONSchema_Overrides_Parameters(t *testing.T) {
	rawSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "File path",
			},
			"options": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"encoding": map[string]interface{}{"type": "string"},
				},
			},
		},
		"required": []string{"path"},
	}

	tool := &Tool{
		Name:          "mcp.fs.read_file",
		Description:   "[MCP:fs] Read file",
		Parameters:    []ToolParam{{Name: "path", Type: "string", Required: true}},
		RawJSONSchema: rawSchema,
	}

	schema := tool.ToJSONSchema()

	// name and description should be present
	if schema["name"] != "mcp.fs.read_file" {
		t.Fatalf("expected name=mcp.fs.read_file, got %v", schema["name"])
	}
	if schema["description"] != "[MCP:fs] Read file" {
		t.Fatalf("expected description, got %v", schema["description"])
	}

	// parameters should be the raw schema, not built from ToolParam
	params := schema["parameters"].(map[string]interface{})
	props := params["properties"].(map[string]interface{})
	if _, ok := props["options"]; !ok {
		t.Fatal("RawJSONSchema should preserve nested 'options' property")
	}
	if _, ok := props["path"]; !ok {
		t.Fatal("RawJSONSchema should preserve 'path' property")
	}

	// Verify it also works through ToOpenAISchema
	oai := tool.ToOpenAISchema()
	if oai["type"] != "function" {
		t.Fatal("expected type=function in OpenAI schema")
	}
	fn := oai["function"].(map[string]interface{})
	if fn["name"] != "mcp.fs.read_file" {
		t.Fatal("OpenAI schema function name mismatch")
	}
}

func TestTool_RawJSONSchema_Nil_Fallback(t *testing.T) {
	tool := &Tool{
		Name:        "local_tool",
		Description: "A local tool",
		Parameters: []ToolParam{
			{Name: "x", Type: "string", Description: "param x", Required: true},
			{Name: "y", Type: "integer", Required: false, Default: 5},
		},
		// RawJSONSchema is nil — should use ToolParam logic
	}

	schema := tool.ToJSONSchema()

	if schema["name"] != "local_tool" {
		t.Fatalf("expected name=local_tool, got %v", schema["name"])
	}

	params := schema["parameters"].(map[string]interface{})
	if params["type"] != "object" {
		t.Fatalf("expected parameters.type=object, got %v", params["type"])
	}

	props := params["properties"].(map[string]interface{})
	if _, ok := props["x"]; !ok {
		t.Fatal("should have property x")
	}
	if _, ok := props["y"]; !ok {
		t.Fatal("should have property y")
	}

	req := params["required"].([]string)
	if len(req) != 1 || req[0] != "x" {
		t.Fatalf("expected required=[x], got %v", req)
	}
}
