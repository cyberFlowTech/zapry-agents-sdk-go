package persona

import (
	"fmt"

)

// NormalizationWarning is a non-fatal issue found during normalization.
type NormalizationWarning struct {
	Field   string
	Message string
}

// Normalize validates and fills defaults for a PersonaSpec.
func Normalize(spec *PersonaSpec) (*PersonaSpec, []NormalizationWarning, error) {
	var warnings []NormalizationWarning

	// Required fields
	if spec.Name == "" {
		return nil, nil, fmt.Errorf("name is required")
	}
	if len(spec.Traits) == 0 {
		return nil, nil, fmt.Errorf("at least one trait is required")
	}

	// Copy to avoid mutating input
	normalized := *spec

	// Defaults
	normalized.Defaults()

	// Locale check: v1 only supports zh-CN
	if normalized.Locale != "zh-CN" {
		warnings = append(warnings, NormalizationWarning{
			Field:   "locale",
			Message: fmt.Sprintf("v1 only supports zh-CN, got %q; using zh-CN", normalized.Locale),
		})
		normalized.Locale = "zh-CN"
	}

	// Traits: cap at 5
	if len(normalized.Traits) > 5 {
		warnings = append(warnings, NormalizationWarning{
			Field:   "traits",
			Message: fmt.Sprintf("traits capped at 5, got %d", len(normalized.Traits)),
		})
		normalized.Traits = normalized.Traits[:5]
	}

	// Hobbies: cap at 5
	if len(normalized.Hobbies) > 5 {
		normalized.Hobbies = normalized.Hobbies[:5]
	}

	// Validate relationship_style
	validStyles := map[string]bool{"friend": true, "mentor": true, "playful": true, "listener": true}
	if !validStyles[normalized.RelationshipStyle] {
		warnings = append(warnings, NormalizationWarning{
			Field:   "relationship_style",
			Message: fmt.Sprintf("unknown style %q, defaulting to friend", normalized.RelationshipStyle),
		})
		normalized.RelationshipStyle = "friend"
	}

	// Validate tone
	validTones := map[string]bool{"warm": true, "calm": true, "sharp": true}
	if !validTones[normalized.Tone] {
		warnings = append(warnings, NormalizationWarning{
			Field:   "tone",
			Message: fmt.Sprintf("unknown tone %q, defaulting to warm", normalized.Tone),
		})
		normalized.Tone = "warm"
	}

	return &normalized, warnings, nil
}
