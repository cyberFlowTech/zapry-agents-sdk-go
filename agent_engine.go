package agentsdk

import (
	"context"
	"log"
	"time"
)

// HandoffEngineConfig is the unified handoff execution engine.
type HandoffEngineConfig struct {
	Registry       *AgentRegistryStore
	Policy         *HandoffPolicyConfig
	Tracer         *AgentTracer
	Cache          *HandoffIdempotencyCache
	PlatformFilter func(*HandoffContextData) *HandoffContextData
}

func NewHandoffEngine(registry *AgentRegistryStore, policy *HandoffPolicyConfig) *HandoffEngineConfig {
	if policy == nil {
		policy = NewHandoffPolicy()
	}
	return &HandoffEngineConfig{Registry: registry, Policy: policy}
}

// Handoff executes the unified handoff pipeline.
func (e *HandoffEngineConfig) Handoff(ctx context.Context, req *HandoffRequestData) *HandoffResultData {
	start := time.Now()

	// 1. Find target
	target := e.Registry.Get(req.ToAgent)
	if target == nil {
		return handoffErrorResult(req, "NOT_FOUND", "Agent not found: "+req.ToAgent)
	}

	// 2. Permission check
	if err := e.Policy.CheckAccess(req, &target.Card); err != nil {
		return handoffErrorResult(req, err.Code, err.Message)
	}

	// 3. Loop guard
	if err := e.Policy.CheckLoop(req); err != nil {
		return handoffErrorResult(req, err.Code, err.Message)
	}

	// 4. Context filtering (platform first, then target)
	hctx := &req.Context
	if e.PlatformFilter != nil {
		hctx = e.PlatformFilter(hctx)
	}
	if target.InputFilter != nil {
		hctx = target.InputFilter(hctx)
	}

	// 5. Timeout
	deadlineMs := req.DeadlineMs
	if deadlineMs <= 0 {
		deadlineMs = e.Policy.DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(deadlineMs)*time.Millisecond)
	defer cancel()

	// 6. Execute target agent
	resultCh := make(chan *HandoffResultData, 1)
	go func() {
		resultCh <- e.runAgent(target, hctx, req)
	}()

	select {
	case result := <-resultCh:
		result.DurationMs = float64(time.Since(start).Milliseconds())
		result.RequestID = req.RequestID

		// Cache if idempotency
		if e.Cache != nil {
			result, _ = e.Cache.GetOrSet(req.RequestID, result)
		}

		return result
	case <-ctx.Done():
		return &HandoffResultData{
			AgentID:   req.ToAgent,
			Status:    "timeout",
			Error:     &HandoffErrorData{Code: "TIMEOUT", Message: "Handoff timed out", Retryable: true},
			DurationMs: float64(time.Since(start).Milliseconds()),
			RequestID: req.RequestID,
		}
	}
}

func (e *HandoffEngineConfig) runAgent(target *AgentRuntimeConfig, hctx *HandoffContextData, req *HandoffRequestData) *HandoffResultData {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[HandoffEngine] Agent %s panic: %v", req.ToAgent, r)
		}
	}()

	if target.LLMFn == nil {
		return handoffErrorResult(req, "MODEL_ERROR", "Agent has no LLM function")
	}

	// Build messages for AgentLoop
	var history []map[string]interface{}
	for _, m := range hctx.Messages {
		history = append(history, map[string]interface{}{"role": m.Role, "content": m.Content})
	}

	userInput := req.Reason
	for i := len(hctx.Messages) - 1; i >= 0; i-- {
		if hctx.Messages[i].Role == "user" {
			userInput = hctx.Messages[i].Content
			break
		}
	}

	toolReg := target.ToolReg
	if toolReg == nil {
		toolReg = NewToolRegistry()
	}

	loop := NewAgentLoop(target.LLMFn, toolReg, target.SystemPrompt, target.MaxTurns, nil)
	loop.Guardrails = target.Guardrails
	loop.Tracer = target.Tracer

	result := loop.Run(userInput, history, hctx.MemorySummary)

	return &HandoffResultData{
		Output:       result.FinalOutput,
		AgentID:      req.ToAgent,
		ShouldReturn: true,
		Status:       "success",
		Usage:        map[string]interface{}{"total_turns": result.TotalTurns, "tool_calls": result.ToolCallsCount},
	}
}
