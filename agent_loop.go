package agentsdk

import (
	"encoding/json"
	"fmt"
	"log"
)

// ──────────────────────────────────────────────
// Agent Loop — ReAct automatic reasoning cycle
// ──────────────────────────────────────────────
//
// Core flow: User Input → LLM → [tool_calls?] → Execute → Feed back → LLM → ... → Final Output
//
// Usage:
//
//	loop := agentsdk.NewAgentLoop(myLLMFn, registry, "You are a helpful assistant.", 10, nil)
//	result := loop.Run("What's the weather in Shanghai?", nil, "")
//	fmt.Println(result.FinalOutput)

// LLMMessage represents a response from the LLM.
type LLMMessage struct {
	Content   string          `json:"content"`
	ToolCalls []ToolCallInput `json:"tool_calls,omitempty"`
}

// LLMFunc is the function signature for calling the LLM.
// It receives the message history and optional tools schema,
// and returns an LLMMessage.
type LLMFunc func(messages []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error)

// ToolCallRecord records a single tool invocation.
type ToolCallRecord struct {
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments"`
	Result    string                 `json:"result"`
	Error     string                 `json:"error,omitempty"`
	CallID    string                 `json:"call_id"`
}

// TurnRecord records a single LLM turn.
type TurnRecord struct {
	TurnNumber int              `json:"turn_number"`
	LLMOutput  string           `json:"llm_output,omitempty"`
	ToolCalls  []ToolCallRecord `json:"tool_calls,omitempty"`
	IsFinal    bool             `json:"is_final"`
}

// AgentLoopResult is the final result of an AgentLoop run.
type AgentLoopResult struct {
	FinalOutput    string                   `json:"final_output"`
	Turns          []TurnRecord             `json:"turns"`
	ToolCallsCount int                      `json:"tool_calls_count"`
	TotalTurns     int                      `json:"total_turns"`
	StoppedReason  string                   `json:"stopped_reason"` // "completed", "max_turns", "error"
	Messages       []map[string]interface{} `json:"messages"`
}

// AgentLoopHooks provides optional event callbacks.
type AgentLoopHooks struct {
	OnLLMStart  func(turn int, messages []map[string]interface{})
	OnLLMEnd    func(turn int, response *LLMMessage)
	OnToolStart func(name string, args map[string]interface{})
	OnToolEnd   func(name string, result string, err string)
	OnTurnEnd   func(turn *TurnRecord)
	OnError     func(err error)
}

// AgentLoop implements the ReAct reasoning cycle.
type AgentLoop struct {
	LLMFn        LLMFunc
	ToolRegistry *ToolRegistry
	SystemPrompt string
	MaxTurns     int
	Hooks        *AgentLoopHooks
}

// NewAgentLoop creates a new agent loop.
func NewAgentLoop(llmFn LLMFunc, registry *ToolRegistry, systemPrompt string, maxTurns int, hooks *AgentLoopHooks) *AgentLoop {
	if maxTurns <= 0 {
		maxTurns = 10
	}
	if hooks == nil {
		hooks = &AgentLoopHooks{}
	}
	return &AgentLoop{
		LLMFn:        llmFn,
		ToolRegistry: registry,
		SystemPrompt: systemPrompt,
		MaxTurns:     maxTurns,
		Hooks:        hooks,
	}
}

// Run executes the agent loop.
func (a *AgentLoop) Run(userInput string, conversationHistory []map[string]interface{}, extraContext string) *AgentLoopResult {
	// Build initial messages
	var messages []map[string]interface{}

	if a.SystemPrompt != "" {
		messages = append(messages, map[string]interface{}{"role": "system", "content": a.SystemPrompt})
	}
	if extraContext != "" {
		messages = append(messages, map[string]interface{}{"role": "system", "content": extraContext})
	}
	if conversationHistory != nil {
		messages = append(messages, conversationHistory...)
	}
	messages = append(messages, map[string]interface{}{"role": "user", "content": userInput})

	// Get tools schema
	var toolsSchema []map[string]interface{}
	if a.ToolRegistry != nil && a.ToolRegistry.Len() > 0 {
		toolsSchema = a.ToolRegistry.ToOpenAISchema()
	}

	result := &AgentLoopResult{}
	turnNumber := 0

	for turnNumber < a.MaxTurns {
		turnNumber++
		turn := TurnRecord{TurnNumber: turnNumber}

		// --- LLM Call ---
		if a.Hooks.OnLLMStart != nil {
			a.Hooks.OnLLMStart(turnNumber, messages)
		}

		llmResp, err := a.LLMFn(messages, toolsSchema)
		if err != nil {
			log.Printf("[AgentLoop] LLM error at turn %d: %v", turnNumber, err)
			if a.Hooks.OnError != nil {
				a.Hooks.OnError(err)
			}
			result.StoppedReason = "error"
			result.FinalOutput = fmt.Sprintf("Error: %v", err)
			break
		}

		if a.Hooks.OnLLMEnd != nil {
			a.Hooks.OnLLMEnd(turnNumber, llmResp)
		}

		turn.LLMOutput = llmResp.Content

		// --- Check: Final output (no tool calls) ---
		if len(llmResp.ToolCalls) == 0 {
			turn.IsFinal = true
			result.FinalOutput = llmResp.Content
			result.StoppedReason = "completed"
			result.Turns = append(result.Turns, turn)
			if a.Hooks.OnTurnEnd != nil {
				a.Hooks.OnTurnEnd(&turn)
			}
			break
		}

		// --- Execute tool calls ---
		assistantMsg := map[string]interface{}{
			"role":    "assistant",
			"content": llmResp.Content,
		}
		// Serialize tool_calls for message history
		var serializedCalls []map[string]interface{}
		for _, tc := range llmResp.ToolCalls {
			serializedCalls = append(serializedCalls, map[string]interface{}{
				"id":   tc.ID,
				"type": "function",
				"function": map[string]string{
					"name":      tc.Function.Name,
					"arguments": tc.Function.Arguments,
				},
			})
		}
		assistantMsg["tool_calls"] = serializedCalls
		messages = append(messages, assistantMsg)

		for _, tc := range llmResp.ToolCalls {
			funcName := tc.Function.Name
			var funcArgs map[string]interface{}
			json.Unmarshal([]byte(tc.Function.Arguments), &funcArgs)
			if funcArgs == nil {
				funcArgs = make(map[string]interface{})
			}

			if a.Hooks.OnToolStart != nil {
				a.Hooks.OnToolStart(funcName, funcArgs)
			}

			record := ToolCallRecord{
				ToolName:  funcName,
				Arguments: funcArgs,
				CallID:    tc.ID,
			}

			// Execute
			ctx := &ToolContext{ToolName: funcName, CallID: tc.ID, Extra: make(map[string]interface{})}
			toolResult, toolErr := a.ToolRegistry.Execute(funcName, funcArgs, ctx)

			var toolResultStr string
			if toolErr != nil {
				record.Error = toolErr.Error()
				toolResultStr = fmt.Sprintf("Error: %v", toolErr)
				log.Printf("[AgentLoop] Tool %s failed: %v", funcName, toolErr)
			} else {
				switch v := toolResult.(type) {
				case string:
					toolResultStr = v
				default:
					b, _ := json.Marshal(v)
					toolResultStr = string(b)
				}
				record.Result = toolResultStr
			}

			if a.Hooks.OnToolEnd != nil {
				a.Hooks.OnToolEnd(funcName, record.Result, record.Error)
			}

			turn.ToolCalls = append(turn.ToolCalls, record)
			result.ToolCallsCount++

			// Append tool result to messages
			messages = append(messages, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": tc.ID,
				"content":      toolResultStr,
			})
		}

		result.Turns = append(result.Turns, turn)
		if a.Hooks.OnTurnEnd != nil {
			a.Hooks.OnTurnEnd(&turn)
		}
	}

	// Check max_turns
	if turnNumber >= a.MaxTurns && result.StoppedReason == "" {
		result.StoppedReason = "max_turns"
		if len(result.Turns) > 0 && result.Turns[len(result.Turns)-1].LLMOutput != "" {
			result.FinalOutput = result.Turns[len(result.Turns)-1].LLMOutput
		}
	}

	result.TotalTurns = turnNumber
	result.Messages = messages
	return result
}
