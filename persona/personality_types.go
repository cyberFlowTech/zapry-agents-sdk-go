// Package personality defines personality system mappings (e.g., MBTI).
package persona

// TraitProfile is the output of a personality mapping.
type TraitProfile struct {
	TraitSupplement []string // additional traits to merge
	TopicPreference []string // preferred conversation topics
	Dislike         []string // disliked topics/styles
	ThinkingStyle   string   // injected into system prompt
	HumorStyle      string   // dry|warm|playful|rare
}

// StyleBias adjusts the style policy.
type StyleBias struct {
	ShareStyle string // "analytical"|"emotional"|"observational"
}

// StateBias adjusts the state machine activity weights.
type StateBias struct {
	PreferActivities []string // boost these activities
	AvoidActivities  []string // demote these activities
}

// ParamBias adjusts model parameters (delta values).
type ParamBias struct {
	TemperatureDelta     float64
	PresencePenaltyDelta float64
}

// MBTIProfile is the complete mapping for one MBTI type.
type MBTIProfile struct {
	Code             string
	RecommendedStyle string // relationship_style recommendation
	Traits           TraitProfile
	StyleBias        StyleBias
	StateBias        StateBias
	ParamBias        ParamBias
}
