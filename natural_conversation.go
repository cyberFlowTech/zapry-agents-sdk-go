package agentsdk

import (
	"context"
	"fmt"
	"time"

	"github.com/cyberFlowTech/zapry-agents-sdk-go/persona"
)

// ──────────────────────────────────────────────
// NaturalConversation — unified entry point for all natural dialogue enhancements
// ──────────────────────────────────────────────

// NaturalConversationConfig controls which capabilities are enabled.
type NaturalConversationConfig struct {
	// Recommended (default ON, zero LLM cost)
	StateTracking    bool // default true
	EmotionDetection bool // default true
	StylePostProcess bool // default true

	// Advanced (default OFF, may incur LLM cost)
	OpenerGeneration bool // default false
	ContextCompress  bool // default false
	StyleRetry       bool // default false

	// Persona (optional, nil = no persona)
	PersonaConfig *persona.RuntimeConfig // compiled persona config
	PersonaTicker *persona.LocalTicker   // local ticker for time-aware state

	// Sub-configs
	StyleConfig      StyleConfig
	OpenerConfig     OpenerConfig
	CompressorConfig CompressorConfig
	SummarizeFn      SummarizeFn // required when ContextCompress=true
	Timezone         string      // default "Asia/Shanghai"
	FollowUpWindow   time.Duration
}

// DefaultNaturalConversationConfig returns the Recommended baseline.
func DefaultNaturalConversationConfig() NaturalConversationConfig {
	return NaturalConversationConfig{
		StateTracking:    true,
		EmotionDetection: true,
		StylePostProcess: true,
		OpenerGeneration: false,
		ContextCompress:  false,
		StyleRetry:       false,
		StyleConfig:      DefaultStyleConfig(),
		OpenerConfig:     DefaultOpenerConfig(),
		CompressorConfig: DefaultCompressorConfig(),
		Timezone:         "Asia/Shanghai",
		FollowUpWindow:   60 * time.Second,
	}
}

// NaturalConversation orchestrates all natural dialogue capabilities.
type NaturalConversation struct {
	config        NaturalConversationConfig
	stateTracker  *ConversationStateTracker
	emotionDet    *EmotionalToneDetector
	styleCtrl     *ResponseStyleController
	opener        *OpenerGenerator
	compressor    *ContextCompressor
	personaConfig *persona.RuntimeConfig
	personaTicker *persona.LocalTicker
}

// NewNaturalConversation creates the enhancement pipeline.
func NewNaturalConversation(config NaturalConversationConfig) *NaturalConversation {
	nc := &NaturalConversation{config: config}

	// Persona: merge blocked phrases into StyleConfig
	if config.PersonaConfig != nil {
		nc.personaConfig = config.PersonaConfig
		nc.personaTicker = config.PersonaTicker
		// Build style constraints to get the blocked phrases list
		sc := persona.BuildStyleConstraints(config.PersonaConfig.StylePolicy)
		if len(sc.BlockedPhrases) > 0 {
			config.StyleConfig.ForbiddenPhrases = mergeUniqueStrings(
				config.StyleConfig.ForbiddenPhrases, sc.BlockedPhrases,
			)
		}
	}

	if config.StateTracking {
		nc.stateTracker = NewConversationStateTracker(config.Timezone)
	}
	if config.EmotionDetection {
		nc.emotionDet = NewEmotionalToneDetector()
	}
	if config.StylePostProcess || config.StyleRetry {
		nc.styleCtrl = NewResponseStyleController(config.StyleConfig)
	}
	if config.OpenerGeneration {
		nc.opener = NewOpenerGenerator(config.OpenerConfig)
	}
	if config.ContextCompress && config.SummarizeFn != nil {
		nc.compressor = NewContextCompressor(config.SummarizeFn, config.CompressorConfig)
	}

	return nc
}

func mergeUniqueStrings(a, b []string) []string {
	seen := make(map[string]bool, len(a))
	for _, s := range a {
		seen[s] = true
	}
	result := append([]string{}, a...)
	for _, s := range b {
		if !seen[s] {
			result = append(result, s)
			seen[s] = true
		}
	}
	return result
}

// Enhance runs all pre-processing enhancements before AgentLoop.Run.
// Returns PromptFragments (for extra_context) and optionally compressed history.
func (nc *NaturalConversation) Enhance(
	session *MemorySession,
	userInput string,
	history []map[string]interface{},
	now time.Time,
) (*PromptFragments, []map[string]interface{}) {
	fragments := NewPromptFragments()
	enhancedHistory := history

	// 0. Persona Tick (time-aware mood, activity, style constraints)
	if nc.personaTicker != nil && nc.personaConfig != nil {
		tick := nc.personaTicker.Tick(nc.personaConfig, session.UserID, now, nil)
		if tick.PromptInjection != "" {
			fragments.AddSystem(tick.PromptInjection)
		}
		if tick.StyleConstraintsText != "" {
			fragments.AddSystem(tick.StyleConstraintsText)
		}
		fragments.SetKV("sdk.persona.mood", tick.CurrentState.Mood)
		fragments.SetKV("sdk.persona.activity", tick.CurrentState.Activity)
		fragments.SetKV("sdk.persona.energy", tick.CurrentState.Energy)
		fragments.AddWarning("persona.tick:" + tick.CurrentState.Mood)
	}

	// 1. State Tracking
	var state *ConversationState
	if nc.stateTracker != nil {
		state = nc.stateTracker.Track(session, userInput, now)

		// Touch session on first turn
		if state.TurnIndex == 1 {
			nc.stateTracker.TouchSession(session, now)
		}

		fragments.AddSystem(state.FormatForPrompt())
		for k, v := range state.ToKV() {
			fragments.SetKV(k, v)
		}
		fragments.AddWarning("state.tracked")
	}

	// 2. Emotion Detection
	if nc.emotionDet != nil {
		tone := nc.emotionDet.Detect(userInput, state)
		if prompt := tone.FormatForPrompt(); prompt != "" {
			fragments.AddSystem(prompt)
			fragments.AddWarning("tone." + tone.Tone + ":" + fmt.Sprintf("%.2f", tone.Confidence))
		}
		fragments.SetKV("sdk.user.emotion_tone", tone.Tone)
		fragments.SetKV("sdk.user.emotion_confidence", tone.Confidence)
	}

	// 3. Opener
	if nc.opener != nil && state != nil {
		openerCount := session.Working.GetInt("sdk.opener_count")
		strategy := nc.opener.Generate(state, openerCount)
		if prompt := strategy.FormatForPrompt(); prompt != "" {
			fragments.AddSystem(prompt)
			session.Working.Incr("sdk.opener_count")
			fragments.AddWarning("opener." + strategy.Situation)
		}
	}

	// 4. Style Prompt
	if nc.styleCtrl != nil {
		if prompt := nc.styleCtrl.BuildStylePrompt(); prompt != "" {
			fragments.AddSystem(prompt)
			fragments.AddWarning("style.prompt:preferred_" + fmt.Sprintf("%d", nc.config.StyleConfig.PreferredLength))
		}
	}

	// 5. Context Compression
	if nc.compressor != nil {
		compressed, err := nc.compressor.Compress(history, session.Working)
		if err == nil && len(compressed) != len(history) {
			enhancedHistory = compressed
			fragments.AddWarning("compressor.summarized")
		}
	}

	return fragments, enhancedHistory
}

// PostProcess applies local style corrections to LLM output.
// Returns corrected text and whether changes were made.
func (nc *NaturalConversation) PostProcess(output string) (string, bool) {
	if nc.styleCtrl == nil {
		return output, false
	}
	result, changed, _ := nc.styleCtrl.PostProcess(output)
	return result, changed
}

// BuildRetryPrompt generates a retry prompt if StyleRetry is enabled.
// Returns nil if no retry needed.
func (nc *NaturalConversation) BuildRetryPrompt(output string) *string {
	if nc.styleCtrl == nil || !nc.config.StyleRetry {
		return nil
	}
	_, _, violations := nc.styleCtrl.PostProcess(output)
	if len(violations) == 0 {
		return nil
	}
	prompt := nc.styleCtrl.BuildRetryPrompt(output, violations)
	if prompt == "" {
		return nil
	}
	return &prompt
}

// ──────────────────────────────────────────────
// NaturalAgentLoop — WrapLoop mode (recommended)
// ──────────────────────────────────────────────

// NaturalAgentLoop wraps AgentLoop with automatic Enhance + PostProcess.
type NaturalAgentLoop struct {
	inner          *AgentLoop
	nc             *NaturalConversation
	lastFragments  *PromptFragments
}

// WrapLoop wraps an existing AgentLoop with natural conversation enhancements.
func (nc *NaturalConversation) WrapLoop(loop *AgentLoop) *NaturalAgentLoop {
	return &NaturalAgentLoop{inner: loop, nc: nc}
}

// Run executes Enhance → AgentLoop.Run → PostProcess.
func (nl *NaturalAgentLoop) Run(session *MemorySession, userInput string, history []map[string]interface{}) *AgentLoopResult {
	return nl.RunContext(context.Background(), session, userInput, history)
}

// RunContext executes with context support for cancellation.
func (nl *NaturalAgentLoop) RunContext(ctx context.Context, session *MemorySession, userInput string, history []map[string]interface{}) *AgentLoopResult {
	// Override SystemPrompt from Persona if available
	if nl.nc.personaConfig != nil && nl.nc.personaConfig.SystemPrompt != "" {
		nl.inner.SystemPrompt = nl.nc.personaConfig.SystemPrompt
	}

	// Enhance
	fragments, enhancedHistory := nl.nc.Enhance(session, userInput, history, time.Now())
	nl.lastFragments = fragments

	// Run
	result := nl.inner.RunContext(ctx, userInput, enhancedHistory, fragments.Text())

	// PostProcess
	if result.StoppedReason == "completed" && result.FinalOutput != "" {
		corrected, changed := nl.nc.PostProcess(result.FinalOutput)
		if changed {
			result.FinalOutput = corrected
		}
	}

	return result
}

// LastFragments returns the PromptFragments from the most recent Run (for debugging).
func (nl *NaturalAgentLoop) LastFragments() *PromptFragments {
	return nl.lastFragments
}

// ──────────────────────────────────────────────
// Hooks mode — for developers who don't want to switch Loop type
// ──────────────────────────────────────────────

// NaturalHooks provides hook functions that can be attached to AgentLoopHooks.
type NaturalHooks struct {
	fragments *PromptFragments
	nc        *NaturalConversation
	session   *MemorySession
}

// BuildHooks creates hooks that integrate NaturalConversation into existing AgentLoopHooks.
// Usage: hooks := nc.BuildHooks(session); loop.Hooks.OnLLMStart = hooks.OnLLMStart
func (nc *NaturalConversation) BuildHooks(session *MemorySession) *NaturalHooks {
	return &NaturalHooks{nc: nc, session: session}
}

// Fragments returns the last computed fragments (for debugging).
func (h *NaturalHooks) Fragments() *PromptFragments {
	return h.fragments
}

