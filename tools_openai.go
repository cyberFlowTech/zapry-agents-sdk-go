package agentsdk

import (
	"encoding/json"
	"fmt"
	"log"
)

// ──────────────────────────────────────────────
// OpenAIToolAdapter — Bridge between ToolRegistry and OpenAI API
// ──────────────────────────────────────────────

// ToolCallInput represents a single tool call from OpenAI's response.
// Maps to the structure in response.choices[0].message.tool_calls[].
type ToolCallInput struct {
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string
	} `json:"function"`
}

// ToolCallResult holds the result of executing a single tool call.
type ToolCallResult struct {
	ToolCallID string `json:"tool_call_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	Error      string `json:"error,omitempty"`
}

// ToMessage converts the result to an OpenAI-compatible tool message.
func (r *ToolCallResult) ToMessage() map[string]string {
	content := r.Content
	if r.Error != "" {
		content = r.Error
	}
	return map[string]string{
		"role":         "tool",
		"tool_call_id": r.ToolCallID,
		"content":      content,
	}
}

// OpenAIToolAdapter adapts a ToolRegistry for use with OpenAI's function calling API.
//
// Usage:
//
//	adapter := agentsdk.NewOpenAIToolAdapter(registry)
//	toolsParam := adapter.ToOpenAITools()     // for API request
//	results := adapter.HandleToolCalls(calls)  // process response
type OpenAIToolAdapter struct {
	Registry *ToolRegistry
}

// NewOpenAIToolAdapter creates a new adapter wrapping the given registry.
func NewOpenAIToolAdapter(registry *ToolRegistry) *OpenAIToolAdapter {
	return &OpenAIToolAdapter{Registry: registry}
}

// ToOpenAITools exports tools in OpenAI tools parameter format.
// Returns a slice suitable for the "tools" field in chat completions.
func (a *OpenAIToolAdapter) ToOpenAITools() []map[string]interface{} {
	return a.Registry.ToOpenAISchema()
}

// HandleToolCalls executes a list of tool calls and returns results.
func (a *OpenAIToolAdapter) HandleToolCalls(calls []ToolCallInput) []ToolCallResult {
	return a.HandleToolCallsWithExtra(calls, nil)
}

// HandleToolCallsWithExtra executes tool calls with additional context.
func (a *OpenAIToolAdapter) HandleToolCallsWithExtra(
	calls []ToolCallInput,
	extra map[string]interface{},
) []ToolCallResult {
	results := make([]ToolCallResult, 0, len(calls))

	for _, call := range calls {
		// Parse arguments
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			args = make(map[string]interface{})
		}

		ctx := &ToolContext{
			ToolName: call.Function.Name,
			CallID:   call.ID,
			Extra:    make(map[string]interface{}),
		}
		if extra != nil {
			for k, v := range extra {
				ctx.Extra[k] = v
			}
		}

		result, err := a.Registry.Execute(call.Function.Name, args, ctx)
		if err != nil {
			log.Printf("[OpenAIToolAdapter] Tool call failed: %s -> %v", call.Function.Name, err)
			results = append(results, ToolCallResult{
				ToolCallID: call.ID,
				Name:       call.Function.Name,
				Error:      err.Error(),
			})
			continue
		}

		// Serialize result
		var content string
		switch v := result.(type) {
		case string:
			content = v
		default:
			b, err := json.Marshal(v)
			if err != nil {
				content = fmt.Sprintf("%v", v)
			} else {
				content = string(b)
			}
		}

		results = append(results, ToolCallResult{
			ToolCallID: call.ID,
			Name:       call.Function.Name,
			Content:    content,
		})
	}

	return results
}

// ResultsToMessages converts results to OpenAI tool message format.
func (a *OpenAIToolAdapter) ResultsToMessages(results []ToolCallResult) []map[string]string {
	msgs := make([]map[string]string, len(results))
	for i, r := range results {
		msgs[i] = r.ToMessage()
	}
	return msgs
}
