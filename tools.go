package agentsdk

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// Tool Calling Framework — LLM-agnostic
// ──────────────────────────────────────────────

// ToolContext is passed to tool handlers during execution.
type ToolContext struct {
	ToolName string
	CallID   string
	Extra    map[string]interface{}
	Ctx      context.Context // optional: propagates cancellation/timeout to tool handlers (e.g. MCP)
}

// ToolParam describes a single parameter of a tool.
type ToolParam struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"` // "string", "integer", "number", "boolean", "array", "object"
	Description string      `json:"description,omitempty"`
	Required    bool        `json:"required"`
	Default     interface{} `json:"default,omitempty"`
	Enum        []string    `json:"enum,omitempty"`
}

// ToolHandlerFunc is the signature for tool execution handlers.
type ToolHandlerFunc func(ctx *ToolContext, args map[string]interface{}) (interface{}, error)

// Tool defines a callable tool with metadata and handler.
type Tool struct {
	Name          string
	Description   string
	Parameters    []ToolParam
	Timeout       time.Duration // optional: max execution time for this tool call
	Handler       ToolHandlerFunc
	RawJSONSchema map[string]interface{} // optional: raw JSON Schema for parameters (used by MCP tools to preserve nested/oneOf/enum)
}

// ToJSONSchema exports this tool as a generic JSON Schema object.
// If RawJSONSchema is set (e.g. from MCP), it is used as the "parameters" value
// to preserve nested/oneOf/enum fidelity. Otherwise, parameters are built from ToolParam.
func (t *Tool) ToJSONSchema() map[string]interface{} {
	if t.RawJSONSchema != nil {
		return map[string]interface{}{
			"name":        t.Name,
			"description": t.Description,
			"parameters":  t.RawJSONSchema,
		}
	}

	properties := make(map[string]interface{})
	var required []string

	for _, p := range t.Parameters {
		prop := map[string]interface{}{
			"type": p.Type,
		}
		if p.Description != "" {
			prop["description"] = p.Description
		}
		if p.Default != nil {
			prop["default"] = p.Default
		}
		if len(p.Enum) > 0 {
			prop["enum"] = p.Enum
		}
		properties[p.Name] = prop
		if p.Required {
			required = append(required, p.Name)
		}
	}

	schema := map[string]interface{}{
		"name":        t.Name,
		"description": t.Description,
		"parameters": map[string]interface{}{
			"type":       "object",
			"properties": properties,
		},
	}
	if len(required) > 0 {
		schema["parameters"].(map[string]interface{})["required"] = required
	}
	return schema
}

// ToOpenAISchema exports in OpenAI function calling format.
func (t *Tool) ToOpenAISchema() map[string]interface{} {
	return map[string]interface{}{
		"type":     "function",
		"function": t.ToJSONSchema(),
	}
}

// ──────────────────────────────────────────────
// ToolRegistry
// ──────────────────────────────────────────────

// ToolRegistry manages tool registration, schema export, and execution.
//
// Usage:
//
//	registry := agentsdk.NewToolRegistry()
//	registry.Register(&agentsdk.Tool{
//	    Name:        "get_weather",
//	    Description: "Get weather for a city",
//	    Parameters: []agentsdk.ToolParam{
//	        {Name: "city", Type: "string", Required: true},
//	    },
//	    Handler: func(ctx *agentsdk.ToolContext, args map[string]interface{}) (interface{}, error) {
//	        return fmt.Sprintf("%s: 25°C", args["city"]), nil
//	    },
//	})
//
//	schema := registry.ToJSONSchema()
//	result, err := registry.Execute("get_weather", map[string]interface{}{"city": "Shanghai"}, nil)
type ToolRegistry struct {
	mu                 sync.RWMutex
	tools              map[string]*Tool
	DefaultToolTimeout time.Duration // global timeout fallback (default 30s, set 0 to disable)
}

// NewToolRegistry creates an empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:              make(map[string]*Tool),
		DefaultToolTimeout: 30 * time.Second,
	}
}

// SetDefaultTimeout sets the global execution timeout for tools in this registry.
// Set to 0 to disable timeout by default.
func (r *ToolRegistry) SetDefaultTimeout(timeout time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.DefaultToolTimeout = timeout
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(t *Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[t.Name] = t
	logInfof("[ToolRegistry] Registered: %s", t.Name)
}

// Get retrieves a tool by name.
func (r *ToolRegistry) Get(name string) *Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools[name]
}

// List returns all registered tools.
func (r *ToolRegistry) List() []*Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*Tool, 0, len(r.tools))
	for _, t := range r.tools {
		result = append(result, t)
	}
	return result
}

// Names returns all registered tool names.
func (r *ToolRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	return names
}

// Remove deletes a tool by name.
func (r *ToolRegistry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tools, name)
}

// Len returns the number of registered tools.
func (r *ToolRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}

// Contains checks whether a tool is registered.
func (r *ToolRegistry) Contains(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.tools[name]
	return ok
}

// ─── Schema export ───

// ToJSONSchema exports all tools as a list of JSON Schema objects.
func (r *ToolRegistry) ToJSONSchema() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	schemas := make([]map[string]interface{}, 0, len(r.tools))
	for _, t := range r.tools {
		schemas = append(schemas, t.ToJSONSchema())
	}
	return schemas
}

// ToOpenAISchema exports all tools in OpenAI function calling format.
func (r *ToolRegistry) ToOpenAISchema() []map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()
	schemas := make([]map[string]interface{}, 0, len(r.tools))
	for _, t := range r.tools {
		schemas = append(schemas, t.ToOpenAISchema())
	}
	return schemas
}

// ─── Execution ───

// Execute runs a tool by name with the given arguments.
func (r *ToolRegistry) Execute(name string, args map[string]interface{}, ctx *ToolContext) (interface{}, error) {
	r.mu.RLock()
	t, ok := r.tools[name]
	defaultTimeout := r.DefaultToolTimeout
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrToolNotFound, name)
	}
	if t.Handler == nil {
		return nil, fmt.Errorf("%w: tool %q", ErrToolNoHandler, name)
	}

	if ctx == nil {
		ctx = &ToolContext{
			ToolName: name,
			Extra:    make(map[string]interface{}),
		}
	} else {
		ctx.ToolName = name
		if ctx.Extra == nil {
			ctx.Extra = make(map[string]interface{})
		}
	}

	if args == nil {
		args = make(map[string]interface{})
	}

	// Fill defaults for missing optional params
	for _, p := range t.Parameters {
		if _, exists := args[p.Name]; !exists && !p.Required && p.Default != nil {
			args[p.Name] = p.Default
		}
	}

	// Check required params
	for _, p := range t.Parameters {
		if p.Required {
			if _, exists := args[p.Name]; !exists {
				return nil, fmt.Errorf("%w: tool %q missing required argument: %q", ErrToolMissingRequiredArg, name, p.Name)
			}
		}
	}

	// Build execution context with optional timeout.
	execCtx := ctx.Ctx
	if execCtx == nil {
		execCtx = context.Background()
	}
	timeout := defaultTimeout
	if t.Timeout > 0 {
		timeout = t.Timeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(execCtx, timeout)
		defer cancel()
	}
	callCtx := &ToolContext{
		ToolName: ctx.ToolName,
		CallID:   ctx.CallID,
		Extra:    ctx.Extra,
		Ctx:      execCtx,
	}

	// Fast-path: no cancellation channel to listen on.
	if timeout <= 0 && execCtx.Done() == nil {
		return t.Handler(callCtx, args)
	}

	type execResult struct {
		value interface{}
		err   error
	}
	resultCh := make(chan execResult, 1)
	go func() {
		value, err := t.Handler(callCtx, args)
		resultCh <- execResult{value: value, err: err}
	}()

	select {
	case res := <-resultCh:
		return res.value, res.err
	case <-execCtx.Done():
		if timeout > 0 && errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("%w: tool %q timed out after %s: %v", ErrToolTimeout, name, timeout, execCtx.Err())
		}
		return nil, fmt.Errorf("%w: tool %q canceled: %v", ErrToolCancelled, name, execCtx.Err())
	}
}
