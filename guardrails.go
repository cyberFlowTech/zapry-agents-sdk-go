package agentsdk

import (
	"fmt"
	"log"
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

type guardrailDef struct {
	name string
	fn   GuardrailFunc
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
	g.inputGuards = append(g.inputGuards, guardrailDef{name: name, fn: fn})
}

// AddOutput registers an output guardrail.
func (g *GuardrailManager) AddOutput(name string, fn GuardrailFunc) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.outputGuards = append(g.outputGuards, guardrailDef{name: name, fn: fn})
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
	result := g.checkInputSafe(text, messages, extra)
	if !result.Passed {
		return &InputGuardrailTriggered{GuardrailName: result.GuardrailName, Reason: result.Reason}
	}
	return nil
}

// CheckOutput runs all output guardrails. Returns error (OutputGuardrailTriggered) on failure.
func (g *GuardrailManager) CheckOutput(text string, messages []map[string]interface{}, extra map[string]interface{}) error {
	result := g.checkOutputSafe(text, messages, extra)
	if !result.Passed {
		return &OutputGuardrailTriggered{GuardrailName: result.GuardrailName, Reason: result.Reason}
	}
	return nil
}

// CheckInputSafe runs input guardrails without error (returns result).
func (g *GuardrailManager) CheckInputSafe(text string, messages []map[string]interface{}, extra map[string]interface{}) *GuardrailResultData {
	return g.checkInputSafe(text, messages, extra)
}

// CheckOutputSafe runs output guardrails without error.
func (g *GuardrailManager) CheckOutputSafe(text string, messages []map[string]interface{}, extra map[string]interface{}) *GuardrailResultData {
	return g.checkOutputSafe(text, messages, extra)
}

func (g *GuardrailManager) checkInputSafe(text string, messages []map[string]interface{}, extra map[string]interface{}) *GuardrailResultData {
	g.mu.RLock()
	guards := make([]guardrailDef, len(g.inputGuards))
	copy(guards, g.inputGuards)
	g.mu.RUnlock()
	return g.runGuards(guards, text, messages, extra)
}

func (g *GuardrailManager) checkOutputSafe(text string, messages []map[string]interface{}, extra map[string]interface{}) *GuardrailResultData {
	g.mu.RLock()
	guards := make([]guardrailDef, len(g.outputGuards))
	copy(guards, g.outputGuards)
	g.mu.RUnlock()
	return g.runGuards(guards, text, messages, extra)
}

func (g *GuardrailManager) runGuards(guards []guardrailDef, text string, messages []map[string]interface{}, extra map[string]interface{}) *GuardrailResultData {
	if len(guards) == 0 {
		return &GuardrailResultData{Passed: true}
	}

	if extra == nil {
		extra = make(map[string]interface{})
	}

	ctx := &GuardrailContext{Text: text, Messages: messages, Extra: extra}

	if g.sequential {
		for _, gd := range guards {
			result := g.execOne(gd, ctx)
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
			ch <- resultWithName{result: g.execOne(gd, ctx)}
		}()
	}

	for i := 0; i < len(guards); i++ {
		rn := <-ch
		if !rn.result.Passed {
			return rn.result
		}
	}
	return &GuardrailResultData{Passed: true}
}

func (g *GuardrailManager) execOne(gd guardrailDef, ctx *GuardrailContext) (result *GuardrailResultData) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Guardrail] %s panic: %v", gd.name, r)
			result = &GuardrailResultData{
				Passed:        false,
				Reason:        fmt.Sprintf("guardrail panic: %v", r),
				GuardrailName: gd.name,
			}
		}
	}()
	result = gd.fn(ctx)
	result.GuardrailName = gd.name
	return result
}
