package persona

// PersonaSpec is the developer input for creating a persona.
// Developers fill 12-15 structured fields instead of writing long prompts.
type PersonaSpec struct {
	Name              string         `json:"name"`
	Age               int            `json:"age,omitempty"`
	Profession        string         `json:"profession,omitempty"`
	Traits            []string       `json:"traits"`              // 3-5 personality traits
	Hobbies           []string       `json:"hobbies,omitempty"`   // 3-5 hobbies
	RelationshipStyle string         `json:"relationship_style"`  // friend|mentor|playful|listener
	Tone              string         `json:"tone,omitempty"`      // warm|calm|sharp
	Boundaries        []string       `json:"boundaries,omitempty"`
	SignatureDetails  map[string]any `json:"signature_details,omitempty"`
	Locale            string         `json:"locale,omitempty"` // default: zh-CN

	// MBTI personality system (optional, Week2)
	PersonalitySystem string `json:"personality_system,omitempty"` // "MBTI"
	PersonalityCode   string `json:"personality_code,omitempty"`   // "INTJ"
}

// Defaults fills in default values for optional fields.
func (s *PersonaSpec) Defaults() {
	if s.Tone == "" {
		s.Tone = "warm"
	}
	if s.Locale == "" {
		s.Locale = "zh-CN"
	}
	if s.RelationshipStyle == "" {
		s.RelationshipStyle = "friend"
	}
}
