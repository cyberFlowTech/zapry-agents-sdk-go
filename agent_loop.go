package agentsdk

import (
	"context"
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

// LLMFunc is the function signature for calling the LLM (without context).
// It receives the message history and optional tools schema,
// and returns an LLMMessage.
type LLMFunc func(messages []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error)

// LLMFuncWithContext is the context-aware LLM function signature (recommended).
// Supports cancellation and timeout propagation.
type LLMFuncWithContext func(ctx context.Context, messages []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error)

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
	LLMFn        LLMFunc            // LLM function without context (backwards compatible)
	LLMFnCtx     LLMFuncWithContext // LLM function with context support (preferred, used if set)
	ToolRegistry *ToolRegistry
	SystemPrompt string
	MaxTurns     int
	Hooks        *AgentLoopHooks
	Guardrails   *GuardrailManager
	Tracer       *AgentTracer
	LoopDetector *LoopDetector      // optional: detects repetitive tool call patterns
	Capabilities *AgentCapabilities // optional: if set, enforces tool whitelist via ToolGrant
}

// callLLM invokes the LLM using the context-aware function if available, otherwise falls back to LLMFn.
func (a *AgentLoop) callLLM(ctx context.Context, messages []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
	if a.LLMFnCtx != nil {
		return a.LLMFnCtx(ctx, messages, tools)
	}
	return a.LLMFn(messages, tools)
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

// Run executes the agent loop (backwards compatible, no cancellation).
// For cancellation support, use RunContext instead.
func (a *AgentLoop) Run(userInput string, conversationHistory []map[string]interface{}, extraContext string) *AgentLoopResult {
	return a.RunContext(context.Background(), userInput, conversationHistory, extraContext)
}

// RunContext executes the agent loop with context support for cancellation/timeout.
// When ctx is cancelled, the loop stops at the next check point and returns
// StoppedReason "cancelled".
func (a *AgentLoop) RunContext(ctx context.Context, userInput string, conversationHistory []map[string]interface{}, extraContext string) *AgentLoopResult {
	// --- Tracing: agent span ---
	var agentSpan *TracingSpan
	if a.Tracer != nil && a.Tracer.enabled {
		a.Tracer.NewTrace()
		agentSpan = a.Tracer.AgentSpan("agent_loop")
		defer func() {
			if agentSpan != nil {
				a.Tracer.EndSpan(agentSpan, agentSpan.Status, agentSpan.Error)
			}
		}()
	}

	// --- Check cancellation before starting ---
	if ctx.Err() != nil {
		return &AgentLoopResult{StoppedReason: "cancelled"}
	}

	// --- Input Guardrails ---
	if a.Guardrails != nil && a.Guardrails.InputCount() > 0 {
		if a.Tracer != nil && a.Tracer.enabled {
			gs := a.Tracer.GuardrailSpan("input_guardrails")
			err := a.Guardrails.CheckInput(userInput, nil, nil)
			if err != nil {
				a.Tracer.EndSpan(gs, "error", err.Error())
				if agentSpan != nil {
					agentSpan.Status = "error"
					agentSpan.Error = err.Error()
				}
				return &AgentLoopResult{
					StoppedReason: "guardrail",
					FinalOutput:   err.Error(),
				}
			}
			a.Tracer.EndSpan(gs, "ok", "")
		} else {
			if err := a.Guardrails.CheckInput(userInput, nil, nil); err != nil {
				return &AgentLoopResult{
					StoppedReason: "guardrail",
					FinalOutput:   err.Error(),
				}
			}
		}
	}

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
		// --- Check cancellation at start of each turn ---
		if ctx.Err() != nil {
			result.StoppedReason = "cancelled"
			break
		}

		turnNumber++
		turn := TurnRecord{TurnNumber: turnNumber}

		// --- LLM Call ---
		if a.Hooks.OnLLMStart != nil {
			a.Hooks.OnLLMStart(turnNumber, messages)
		}

		var llmSpan *TracingSpan
		if a.Tracer != nil && a.Tracer.enabled {
			llmSpan = a.Tracer.LLMSpan("", map[string]interface{}{"turn": turnNumber})
		}
		llmResp, err := a.callLLM(ctx, messages, toolsSchema)
		if llmSpan != nil {
			status := "ok"
			errMsg := ""
			if err != nil {
				status = "error"
				errMsg = err.Error()
			}
			a.Tracer.EndSpan(llmSpan, status, errMsg)
		}
		if err != nil {
			// Check if the error is due to context cancellation
			if ctx.Err() != nil {
				result.StoppedReason = "cancelled"
				break
			}
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
			// --- Output Guardrails ---
			if a.Guardrails != nil && a.Guardrails.OutputCount() > 0 && llmResp.Content != "" {
				if a.Tracer != nil && a.Tracer.enabled {
					gs := a.Tracer.GuardrailSpan("output_guardrails")
					err := a.Guardrails.CheckOutput(llmResp.Content, nil, nil)
					if err != nil {
						a.Tracer.EndSpan(gs, "error", err.Error())
						result.StoppedReason = "guardrail"
						result.FinalOutput = err.Error()
						if agentSpan != nil {
							agentSpan.Status = "error"
							agentSpan.Error = err.Error()
						}
						break
					}
					a.Tracer.EndSpan(gs, "ok", "")
				} else if err := a.Guardrails.CheckOutput(llmResp.Content, nil, nil); err != nil {
					result.StoppedReason = "guardrail"
					result.FinalOutput = err.Error()
					break
				}
			}

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

		cancelled := false
		loopDetected := false
		for _, tc := range llmResp.ToolCalls {
			// Check cancellation before each tool execution
			if ctx.Err() != nil {
				cancelled = true
				break
			}

			funcName := tc.Function.Name
			var funcArgs map[string]interface{}
			json.Unmarshal([]byte(tc.Function.Arguments), &funcArgs)

			// Capability enforcement: check ToolGrant before execution
			if decision := CheckToolGrant(a.Capabilities, funcName); !decision.Allowed {
				log.Printf("[AgentLoop] Tool %s denied: %s", funcName, decision.DenyReason)
				messages = append(messages, map[string]interface{}{
					"role":         "tool",
					"tool_call_id": tc.ID,
					"content":      fmt.Sprintf("Error: %s", decision.DenyReason),
				})
				turn.ToolCalls = append(turn.ToolCalls, ToolCallRecord{
					ToolName: funcName, CallID: tc.ID, Error: decision.DenyReason,
				})
				continue
			}
			if funcArgs == nil {
				funcArgs = make(map[string]interface{})
			}

			// Loop detection: check before executing
			if a.LoopDetector != nil {
				if warning := a.LoopDetector.Check(funcName, funcArgs); warning != nil {
					if warning.Type == "repeat" {
						loopDetected = true
						result.FinalOutput = warning.Message
						break
					}
					// flood/ping_pong: inject warning into messages, continue
					messages = append(messages, map[string]interface{}{
						"role":    "system",
						"content": "[Warning] " + warning.Message + ". Try a different approach.",
					})
				}
			}

			if a.Hooks.OnToolStart != nil {
				a.Hooks.OnToolStart(funcName, funcArgs)
			}

			record := ToolCallRecord{
				ToolName:  funcName,
				Arguments: funcArgs,
				CallID:    tc.ID,
			}

			// Execute (with tracing), pass ctx through ToolContext
			var toolSpan *TracingSpan
			if a.Tracer != nil && a.Tracer.enabled {
				toolSpan = a.Tracer.ToolSpan(funcName, funcArgs)
			}
			toolCtx := &ToolContext{ToolName: funcName, CallID: tc.ID, Extra: make(map[string]interface{}), Ctx: ctx}
			toolResult, toolErr := a.ToolRegistry.Execute(funcName, funcArgs, toolCtx)
			if toolSpan != nil {
				status := "ok"
				errMsg := ""
				if toolErr != nil {
					status = "error"
					errMsg = toolErr.Error()
				}
				a.Tracer.EndSpan(toolSpan, status, errMsg)
			}

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

			// Loop detection: record after execution
			if a.LoopDetector != nil {
				a.LoopDetector.Record(funcName, funcArgs)
			}

			messages = append(messages, map[string]interface{}{
				"role":         "tool",
				"tool_call_id": tc.ID,
				"content":      toolResultStr,
			})
		}

		if loopDetected {
			result.StoppedReason = "loop_detected"
			result.Turns = append(result.Turns, turn)
			break
		}

		if cancelled {
			result.StoppedReason = "cancelled"
			result.Turns = append(result.Turns, turn)
			break
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
