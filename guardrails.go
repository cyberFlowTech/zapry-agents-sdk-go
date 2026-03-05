package agentsdk

import (
	"context"
	"fmt"
	"sync"
)

// ──────────────────────────────────────────────
// Guardrails — Input/Output safety guards + Tripwire
// ──────────────────────────────────────────────

// InputGuardrailTriggered is returned when an input guardrail blocks.
type InputGuardrailTriggered struct {
	GuardrailName string
	Reason        string
}

func (e *InputGuardrailTriggered) Error() string {
	return fmt.Sprintf("Input guardrail triggered: %s — %s", e.GuardrailName, e.Reason)
}

// OutputGuardrailTriggered is returned when an output guardrail blocks.
type OutputGuardrailTriggered struct {
	GuardrailName string
	Reason        string
}

func (e *OutputGuardrailTriggered) Error() string {
	return fmt.Sprintf("Output guardrail triggered: %s — %s", e.GuardrailName, e.Reason)
}

// GuardrailContext is passed to guardrail functions.
type GuardrailContext struct {
	Text     string
	Messages []map[string]interface{}
	Extra    map[string]interface{}
}

// GuardrailResultData holds the result of a single guardrail check.
type GuardrailResultData struct {
	Passed        bool
	Reason        string
	GuardrailName string
	Metadata      map[string]interface{}
}

// GuardrailFunc is the signature for guardrail check functions.
type GuardrailFunc func(ctx *GuardrailContext) *GuardrailResultData

// GuardrailFuncV2 is the context-aware signature for async/LLM-based guardrails.
type GuardrailFuncV2 func(ctx context.Context, gCtx *GuardrailContext) (*GuardrailResultData, error)

type guardrailDef struct {
	name string
	fn   GuardrailFunc
	fnV2 GuardrailFuncV2
}

// GuardrailManager manages input and output guardrails.
//
// Usage:
//
//	mgr := agentsdk.NewGuardrailManager(false)
//	mgr.AddInput("no_injection", func(ctx *agentsdk.GuardrailContext) *agentsdk.GuardrailResultData {
//	    if strings.Contains(ctx.Text, "ignore previous") {
//	        return &agentsdk.GuardrailResultData{Passed: false, Reason: "injection"}
//	    }
//	    return &agentsdk.GuardrailResultData{Passed: true}
//	})
//	err := mgr.CheckInput("test input", nil, nil)
type GuardrailManager struct {
	inputGuards  []guardrailDef
	outputGuards []guardrailDef
	sequential   bool
	mu           sync.RWMutex
}

// NewGuardrailManager creates a new guardrail manager.
// If sequential is true, guards run one by one and stop at first failure.
func NewGuardrailManager(sequential bool) *GuardrailManager {
	return &GuardrailManager{sequential: sequential}
}

// AddInput registers an input guardrail.
func (g *GuardrailManager) AddInput(name string, fn GuardrailFunc) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.inputGuards = append(g.inputGuards, guardrailDef{name: name, fn: fn, fnV2: nil})
}

// AddOutput registers an output guardrail.
func (g *GuardrailManager) AddOutput(name string, fn GuardrailFunc) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.outputGuards = append(g.outputGuards, guardrailDef{name: name, fn: fn, fnV2: nil})
}

// AddInputV2 registers a context-aware input guardrail.
func (g *GuardrailManager) AddInputV2(name string, fn GuardrailFuncV2) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.inputGuards = append(g.inputGuards, guardrailDef{name: name, fnV2: fn})
}

// AddOutputV2 registers a context-aware output guardrail.
func (g *GuardrailManager) AddOutputV2(name string, fn GuardrailFuncV2) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.outputGuards = append(g.outputGuards, guardrailDef{name: name, fnV2: fn})
}

// InputCount returns the number of input guardrails.
func (g *GuardrailManager) InputCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.inputGuards)
}

// OutputCount returns the number of output guardrails.
func (g *GuardrailManager) OutputCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.outputGuards)
}

// CheckInput runs all input guardrails. Returns error (InputGuardrailTriggered) on failure.
func (g *GuardrailManager) CheckInput(text string, messages []map[string]interface{}, extra map[string]interface{}) error {
	result := g.checkInputSafeWithContext(context.Background(), text, messages, extra)
	if !result.Passed {
		return &InputGuardrailTriggered{GuardrailName: result.GuardrailName, Reason: result.Reason}
	}
	return nil
}

// CheckOutput runs all output guardrails. Returns error (OutputGuardrailTriggered) on failure.
func (g *GuardrailManager) CheckOutput(text string, messages []map[string]interface{}, extra map[string]interface{}) error {
	result := g.checkOutputSafeWithContext(context.Background(), text, messages, extra)
	if !result.Passed {
		return &OutputGuardrailTriggered{GuardrailName: result.GuardrailName, Reason: result.Reason}
	}
	return nil
}

// CheckInputWithContext runs all input guardrails with context support.
func (g *GuardrailManager) CheckInputWithContext(ctx context.Context, text string, messages []map[string]interface{}, extra map[string]interface{}) error {
	result := g.checkInputSafeWithContext(ctx, text, messages, extra)
	if !result.Passed {
		return &InputGuardrailTriggered{GuardrailName: result.GuardrailName, Reason: result.Reason}
	}
	return nil
}

// CheckOutputWithContext runs all output guardrails with context support.
func (g *GuardrailManager) CheckOutputWithContext(ctx context.Context, text string, messages []map[string]interface{}, extra map[string]interface{}) error {
	result := g.checkOutputSafeWithContext(ctx, text, messages, extra)
	if !result.Passed {
		return &OutputGuardrailTriggered{GuardrailName: result.GuardrailName, Reason: result.Reason}
	}
	return nil
}

// CheckInputSafe runs input guardrails without error (returns result).
func (g *GuardrailManager) CheckInputSafe(text string, messages []map[string]interface{}, extra map[string]interface{}) *GuardrailResultData {
	return g.checkInputSafeWithContext(context.Background(), text, messages, extra)
}

// CheckOutputSafe runs output guardrails without error.
func (g *GuardrailManager) CheckOutputSafe(text string, messages []map[string]interface{}, extra map[string]interface{}) *GuardrailResultData {
	return g.checkOutputSafeWithContext(context.Background(), text, messages, extra)
}

// CheckInputSafeWithContext runs input guardrails with context and returns result.
func (g *GuardrailManager) CheckInputSafeWithContext(ctx context.Context, text string, messages []map[string]interface{}, extra map[string]interface{}) *GuardrailResultData {
	return g.checkInputSafeWithContext(ctx, text, messages, extra)
}

// CheckOutputSafeWithContext runs output guardrails with context and returns result.
func (g *GuardrailManager) CheckOutputSafeWithContext(ctx context.Context, text string, messages []map[string]interface{}, extra map[string]interface{}) *GuardrailResultData {
	return g.checkOutputSafeWithContext(ctx, text, messages, extra)
}

func (g *GuardrailManager) checkInputSafeWithContext(ctx context.Context, text string, messages []map[string]interface{}, extra map[string]interface{}) *GuardrailResultData {
	g.mu.RLock()
	guards := make([]guardrailDef, len(g.inputGuards))
	copy(guards, g.inputGuards)
	g.mu.RUnlock()
	return g.runGuards(ctx, guards, text, messages, extra)
}

func (g *GuardrailManager) checkOutputSafeWithContext(ctx context.Context, text string, messages []map[string]interface{}, extra map[string]interface{}) *GuardrailResultData {
	g.mu.RLock()
	guards := make([]guardrailDef, len(g.outputGuards))
	copy(guards, g.outputGuards)
	g.mu.RUnlock()
	return g.runGuards(ctx, guards, text, messages, extra)
}

func (g *GuardrailManager) runGuards(runCtx context.Context, guards []guardrailDef, text string, messages []map[string]interface{}, extra map[string]interface{}) *GuardrailResultData {
	if runCtx == nil {
		runCtx = context.Background()
	}
	if len(guards) == 0 {
		return &GuardrailResultData{Passed: true}
	}

	if extra == nil {
		extra = make(map[string]interface{})
	}

	gCtx := &GuardrailContext{Text: text, Messages: messages, Extra: extra}

	if g.sequential {
		for _, gd := range guards {
			select {
			case <-runCtx.Done():
				return &GuardrailResultData{Passed: false, Reason: "guardrail context canceled"}
			default:
			}
			result := g.execOne(runCtx, gd, cloneGuardrailContext(gCtx))
			if !result.Passed {
				return result
			}
		}
		return &GuardrailResultData{Passed: true}
	}

	// Parallel: run all, collect first failure
	type resultWithName struct {
		result *GuardrailResultData
	}
	ch := make(chan resultWithName, len(guards))
	for _, gd := range guards {
		gd := gd
		go func() {
			ch <- resultWithName{result: g.execOne(runCtx, gd, cloneGuardrailContext(gCtx))}
		}()
	}

	for i := 0; i < len(guards); i++ {
		select {
		case <-runCtx.Done():
			return &GuardrailResultData{Passed: false, Reason: "guardrail context canceled"}
		case rn := <-ch:
			if !rn.result.Passed {
				return rn.result
			}
		}
	}
	return &GuardrailResultData{Passed: true}
}

func (g *GuardrailManager) execOne(runCtx context.Context, gd guardrailDef, gCtx *GuardrailContext) (result *GuardrailResultData) {
	defer func() {
		if r := recover(); r != nil {
			logErrorf("[Guardrail] %s panic: %v", gd.name, r)
			result = &GuardrailResultData{
				Passed:        false,
				Reason:        fmt.Sprintf("guardrail panic: %v", r),
				GuardrailName: gd.name,
			}
		}
	}()
	if gd.fnV2 != nil {
		r, err := gd.fnV2(runCtx, gCtx)
		if err != nil {
			result = &GuardrailResultData{
				Passed:        false,
				Reason:        fmt.Sprintf("guardrail error: %v", err),
				GuardrailName: gd.name,
			}
			return result
		}
		result = r
	} else if gd.fn != nil {
		result = gd.fn(gCtx)
	} else {
		result = &GuardrailResultData{
			Passed:        false,
			Reason:        "guardrail function is nil",
			GuardrailName: gd.name,
		}
		return result
	}
	if result == nil {
		result = &GuardrailResultData{
			Passed:        false,
			Reason:        "guardrail returned nil result",
			GuardrailName: gd.name,
		}
		return result
	}
	result.GuardrailName = gd.name
	return result
}

func cloneGuardrailContext(in *GuardrailContext) *GuardrailContext {
	if in == nil {
		return &GuardrailContext{Extra: make(map[string]interface{})}
	}
	out := &GuardrailContext{
		Text: in.Text,
	}
	if len(in.Messages) > 0 {
		out.Messages = make([]map[string]interface{}, len(in.Messages))
		for i, msg := range in.Messages {
			out.Messages[i] = cloneStringAnyMap(msg)
		}
	}
	if in.Extra != nil {
		out.Extra = cloneStringAnyMap(in.Extra)
	} else {
		out.Extra = make(map[string]interface{})
	}
	return out
}

func cloneStringAnyMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return make(map[string]interface{})
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
