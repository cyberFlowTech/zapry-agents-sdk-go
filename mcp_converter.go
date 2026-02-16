package agentsdk

import (
	"context"
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────
// MCP Client — Tool conversion (MCP -> SDK Tool)
// ──────────────────────────────────────────────

// mcpCallResult carries both the normalized text and the raw result (for tracing).
type mcpCallResult struct {
	Text string         // concatenated text content for AgentLoop
	Raw  *MCPToolResult // original structure for tracing (when TraceArgs=true)
}

// mcpResultToCallResult normalizes an MCPToolResult into text + raw.
func mcpResultToCallResult(result *MCPToolResult) *mcpCallResult {
	var sb strings.Builder
	for _, c := range result.Content {
		if c.Type == "text" && c.Text != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(c.Text)
		}
	}
	text := sb.String()
	if result.IsError {
		text = "Error: " + text
	}
	return &mcpCallResult{Text: text, Raw: result}
}

// mcpToolName generates the injected SDK tool name: mcp.{server}.{tool}.
func mcpToolName(server, tool string) string {
	return "mcp." + server + "." + tool
}

// extractToolParams extracts top-level ToolParam from an MCP inputSchema for basic validation.
// The full schema is preserved in Tool.RawJSONSchema for LLM consumption.
func extractToolParams(inputSchema map[string]interface{}) []ToolParam {
	if inputSchema == nil {
		return nil
	}
	propsRaw, ok := inputSchema["properties"]
	if !ok {
		return nil
	}
	props, ok := propsRaw.(map[string]interface{})
	if !ok {
		return nil
	}

	requiredSet := map[string]bool{}
	if reqRaw, ok := inputSchema["required"]; ok {
		if reqArr, ok := reqRaw.([]interface{}); ok {
			for _, r := range reqArr {
				if s, ok := r.(string); ok {
					requiredSet[s] = true
				}
			}
		}
	}

	var params []ToolParam
	for name, propRaw := range props {
		prop, ok := propRaw.(map[string]interface{})
		if !ok {
			continue
		}
		p := ToolParam{
			Name:     name,
			Required: requiredSet[name],
		}
		if t, ok := prop["type"].(string); ok {
			p.Type = t
		}
		if d, ok := prop["description"].(string); ok {
			p.Description = d
		}
		params = append(params, p)
	}
	return params
}

// ConvertMCPTools converts MCP tool definitions to SDK *Tool instances.
//
// Design:
//   - Wildcard filtering matches original MCP tool name (not the sdk name)
//   - RawJSONSchema stores the inputSchema as-is (preserves nested/oneOf/enum)
//   - Handler closure propagates context via ToolContext.Ctx
//   - MaxTools truncation applied after filtering
func ConvertMCPTools(
	serverName string,
	mcpTools []MCPToolDef,
	callFn func(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error),
	config *MCPServerConfig,
) []*Tool {
	var tools []*Tool

	for _, mt := range mcpTools {
		if config != nil && !isToolAllowed(mt.Name, config) {
			continue
		}

		originalName := mt.Name
		sdkName := mcpToolName(serverName, originalName)

		handler := func(ctx *ToolContext, args map[string]interface{}) (interface{}, error) {
			callCtx := context.Background()
			if ctx != nil && ctx.Ctx != nil {
				callCtx = ctx.Ctx
			}
			return callFn(callCtx, originalName, args)
		}

		tool := &Tool{
			Name:          sdkName,
			Description:   fmt.Sprintf("[MCP:%s] %s", serverName, mt.Description),
			Parameters:    extractToolParams(mt.InputSchema),
			RawJSONSchema: mt.InputSchema,
			Handler:       handler,
		}
		tools = append(tools, tool)

		if config != nil && config.MaxTools > 0 && len(tools) >= config.MaxTools {
			break
		}
	}

	return tools
}
