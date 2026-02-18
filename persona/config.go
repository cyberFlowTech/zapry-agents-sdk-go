package persona

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// RuntimeConfig is the compiled output of a PersonaSpec.
// It is versioned, storable, and reusable.
type RuntimeConfig struct {
	PersonaID  string `json:"persona_id"`
	Version    string `json:"version"`
	SpecHash   string `json:"spec_hash"`
	ConfigHash string `json:"config_hash"`

	SystemPrompt string       `json:"system_prompt"`
	StylePolicy  StylePolicy  `json:"style_policy"`
	StateMachine StateMachine `json:"state_machine"`
	MoodModel    MoodModel    `json:"mood_model"`
	MemoryPolicy MemoryPolicy `json:"memory_policy"`
	ModelParams  ModelParams  `json:"model_params"`

	PromptBudget     PromptBudget    `json:"prompt_budget"`
	TodayEventPool   []EventTemplate `json:"today_event_pool"`
	SignatureDetails map[string]any  `json:"signature_details,omitempty"`
	Locale           string          `json:"locale"`
	RuntimeMode      string          `json:"runtime_mode"` // "local" | "service"
	PersonalityCode  string          `json:"personality_code,omitempty"`
}

// StylePolicy controls conversational behavior constraints.
type StylePolicy struct {
	ShareFirst              bool `json:"share_first"`
	MaxQuestionsPerTurn     int  `json:"max_questions_per_turn"`
	MaxConsecutiveQuestions  int  `json:"max_consecutive_questions"`
	AllowTopicShift         bool `json:"allow_topic_shift"`
	AvoidInterviewMode      bool `json:"avoid_interview_mode"`
}

// StateMachine defines time-based activity slots.
type StateMachine struct {
	Timezone string     `json:"timezone"`
	Slots    []TimeSlot `json:"slots"`
}

// TimeSlot is a time range with weighted activities.
type TimeSlot struct {
	Range      string    `json:"range"`      // "06:00-09:00"
	Activities []string  `json:"activities"`
	Weights    []float64 `json:"weights"`
}

// MoodModel controls the persona's emotional baseline.
type MoodModel struct {
	BaseMood    string  `json:"base_mood"`    // calm|happy|neutral
	EnergyCurve string  `json:"energy_curve"` // morning_high|night_low|flat
	Reactivity  float64 `json:"reactivity"`   // 0.0-1.0
}

// MemoryPolicy defines what the persona remembers.
type MemoryPolicy struct {
	WriteLongTermWhen []string `json:"write_long_term_when"`
	NeverStore        []string `json:"never_store"`
}

// ModelParams are the default LLM parameters.
type ModelParams struct {
	Temperature      float64 `json:"temperature"`
	PresencePenalty  float64 `json:"presence_penalty"`
	FrequencyPenalty float64 `json:"frequency_penalty"`
	TopP             float64 `json:"top_p"`
	MaxTokens        int     `json:"max_tokens"`
}

// PromptBudget controls token/char injection limits.
type PromptBudget struct {
	MaxSystemPromptChars  int `json:"max_system_prompt_chars"`  // default 1500
	MaxTickInjectionChars int `json:"max_tick_injection_chars"` // default 300
	MaxTotalSystemChars   int `json:"max_total_system_chars"`   // default 3000
}

// DefaultPromptBudget returns the default prompt budget.
func DefaultPromptBudget() PromptBudget {
	return PromptBudget{
		MaxSystemPromptChars:  1500,
		MaxTickInjectionChars: 300,
		MaxTotalSystemChars:   3000,
	}
}

// EventTemplate is a daily event for the today_event_pool.
type EventTemplate struct {
	ID       string  `json:"id"`
	Event    string  `json:"event"`
	Category string  `json:"category"` // work|life|pet|hobby
	Weight   float64 `json:"weight"`
}

// SaveLocal persists the config to a local JSON file.
func (c *RuntimeConfig) SaveLocal(dir string) error {
	configDir := filepath.Join(dir, c.PersonaID)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(filepath.Join(configDir, "config.json"), data, 0644)
}

// LoadLocalConfig loads a RuntimeConfig from a local JSON file.
func LoadLocalConfig(dir string, personaID string) (*RuntimeConfig, error) {
	path := filepath.Join(dir, personaID, "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var config RuntimeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &config, nil
}

// ComputeConfigHash computes the SHA256 hash of the config.
func (c *RuntimeConfig) ComputeConfigHash() string {
	data, _ := json.Marshal(c)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
