package persona

// MoodState represents the persona's current mood.
type MoodState struct {
	Label string  // descriptive label
	Value float64 // numeric value (0-100)
}

// CalculateMood computes mood from base mood and energy.
// v1: simplified, no user signal processing.
func CalculateMood(baseMood string, energy int) MoodState {
	base := baseMoodToNumeric(baseMood)

	// Blend with energy
	energyFactor := float64(energy) / 100.0
	value := base*0.6 + (energyFactor*100)*0.4

	// Clamp
	if value < 0 {
		value = 0
	}
	if value > 100 {
		value = 100
	}

	return MoodState{
		Label: numericToLabel(value, energy),
		Value: value,
	}
}

func baseMoodToNumeric(mood string) float64 {
	switch mood {
	case "happy":
		return 75
	case "calm":
		return 55
	case "neutral":
		return 50
	case "tired":
		return 30
	case "melancholy":
		return 35
	default:
		return 50
	}
}

func numericToLabel(value float64, energy int) string {
	switch {
	case energy < 30 && value < 40:
		return "有点累但心里很平静"
	case energy < 30:
		return "有些疲惫"
	case value >= 70:
		return "心情不错"
	case value >= 50:
		return "挺放松的"
	case value >= 35:
		return "有点懒洋洋的"
	default:
		return "安安静静的"
	}
}
