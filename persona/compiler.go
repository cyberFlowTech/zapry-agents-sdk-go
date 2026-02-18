package persona

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

)

// Compiler compiles a PersonaSpec into a RuntimeConfig.
type Compiler struct{}

// NewCompiler creates a new Compiler.
func NewCompiler() *Compiler {
	return &Compiler{}
}

// Compile runs the full compilation pipeline:
// 1. Normalize → 2. (MBTI Enhance, Week2) → 3. Template Selection →
// 4. Prompt Assembly → 5. Policy Generation → 6. State Machine →
// 7. Event Pool → 8. Param Tuning → 9. Versioning
func (c *Compiler) Compile(spec *PersonaSpec) (*RuntimeConfig, error) {
	// 1. Normalize
	normalized, warnings, err := Normalize(spec)
	if err != nil {
		return nil, fmt.Errorf("normalize: %w", err)
	}
	_ = warnings // log in production

	// 2. MBTI Enhance
	mbtiEnhancer := NewMBTIEnhancer()
	enhanced, mbtiProfile := mbtiEnhancer.Enhance(normalized)
	normalized = enhanced

	// 3. Template Selection
	tpl := GetTemplate(normalized.RelationshipStyle)

	// 4. Prompt Assembly (with MBTI thinking style injection)
	var thinkingStyle string
	if mbtiProfile != nil {
		thinkingStyle = mbtiProfile.Traits.ThinkingStyle
	}
	systemPrompt := AssemblePrompt(normalized, tpl, thinkingStyle)

	// 5. Policy Generation
	stylePolicy := tpl.DefaultStylePolicy
	memoryPolicy := buildMemoryPolicy(normalized)

	// 6. State Machine Generation
	stateMachine := GenerateStateMachine(normalized)

	// 7. Event Pool Generation
	eventPool := GenerateEventPool(normalized)

	// 8. Param Tuning (with MBTI bias)
	modelParams := tuneParams(normalized, tpl)
	if mbtiProfile != nil {
		modelParams.Temperature += mbtiProfile.ParamBias.TemperatureDelta
		modelParams.PresencePenalty += mbtiProfile.ParamBias.PresencePenaltyDelta
		// Clamp
		if modelParams.Temperature < 0.5 {
			modelParams.Temperature = 0.5
		}
		if modelParams.Temperature > 1.0 {
			modelParams.Temperature = 1.0
		}
	}

	// 9. Versioning
	personaID := generatePersonaID()
	specHash := computeSpecHash(normalized)

	config := &RuntimeConfig{
		PersonaID:    personaID,
		Version:      "1.0.0",
		SpecHash:     specHash,
		SystemPrompt: systemPrompt,
		StylePolicy:  stylePolicy,
		StateMachine: stateMachine,
		MoodModel: MoodModel{
			BaseMood:    "calm",
			EnergyCurve: "night_low",
			Reactivity:  0.6,
		},
		MemoryPolicy:    memoryPolicy,
		ModelParams:     modelParams,
		PromptBudget:     DefaultPromptBudget(),
		TodayEventPool:   eventPool,
		SignatureDetails: normalized.SignatureDetails,
		Locale:           normalized.Locale,
		RuntimeMode:      "local",
		PersonalityCode:  normalized.PersonalityCode,
	}

	config.ConfigHash = config.ComputeConfigHash()
	return config, nil
}

func buildMemoryPolicy(spec *PersonaSpec) MemoryPolicy {
	return MemoryPolicy{
		WriteLongTermWhen: []string{"user_preference", "relationship_milestone"},
		NeverStore:        []string{"password", "seed_phrase", "id_number"},
	}
}

func tuneParams(spec *PersonaSpec, tpl *Template) ModelParams {
	params := tpl.DefaultModelParams

	// Tone-based adjustments
	switch spec.Tone {
	case "calm":
		params.Temperature -= 0.10
		params.PresencePenalty -= 0.2
	case "sharp":
		params.Temperature -= 0.15
		params.PresencePenalty -= 0.1
	}

	// Clamp values
	if params.Temperature < 0.5 {
		params.Temperature = 0.5
	}
	if params.Temperature > 1.0 {
		params.Temperature = 1.0
	}

	return params
}

func generatePersonaID() string {
	// UUIDv7-like: time-based prefix for ordering
	now := time.Now().UnixMilli()
	return fmt.Sprintf("p_%x", now)
}

func computeSpecHash(spec *PersonaSpec) string {
	data, _ := json.Marshal(spec)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
