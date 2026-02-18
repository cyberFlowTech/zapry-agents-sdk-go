package persona

import (
)

// activityTimeMap maps common hobbies to their natural time slots.
var activityTimeMap = map[string]string{
	"瑜伽":   "morning",
	"跑步":   "morning",
	"晨练":   "morning",
	"手冲咖啡": "morning",
	"咖啡":   "morning",
	"读书":   "evening",
	"看书":   "evening",
	"阅读":   "evening",
	"音乐":   "evening",
	"听音乐":  "evening",
	"猫":    "evening",
	"宠物":   "evening",
	"做饭":   "noon",
	"午餐":   "noon",
	"散步":   "afternoon",
	"逛街":   "afternoon",
	"游戏":   "night",
	"写作":   "afternoon",
}

// GenerateStateMachine creates time-based activity slots from the spec.
func GenerateStateMachine(spec *PersonaSpec) StateMachine {
	sm := StateMachine{
		Timezone: "Asia/Shanghai", // v1: default from locale
	}

	// Categorize hobbies into time slots
	morningActs := []string{}
	noonActs := []string{}
	afternoonActs := []string{}
	eveningActs := []string{}
	nightActs := []string{}

	for _, hobby := range spec.Hobbies {
		slot, ok := activityTimeMap[hobby]
		if !ok {
			// Default: spread across afternoon and evening
			afternoonActs = append(afternoonActs, hobby)
			continue
		}
		switch slot {
		case "morning":
			morningActs = append(morningActs, hobby)
		case "noon":
			noonActs = append(noonActs, hobby)
		case "afternoon":
			afternoonActs = append(afternoonActs, hobby)
		case "evening":
			eveningActs = append(eveningActs, hobby)
		case "night":
			nightActs = append(nightActs, hobby)
		}
	}

	// Add profession-based work activities
	if spec.Profession != "" {
		noonActs = append(noonActs, "工作")
		afternoonActs = append(afternoonActs, "工作")
	}

	// Ensure every slot has at least one activity
	if len(morningActs) == 0 {
		morningActs = []string{"醒来", "准备出门"}
	}
	if len(noonActs) == 0 {
		noonActs = []string{"午餐", "休息"}
	}
	if len(afternoonActs) == 0 {
		afternoonActs = []string{"忙碌中"}
	}
	if len(eveningActs) == 0 {
		eveningActs = []string{"放松", "看手机"}
	}
	if len(nightActs) == 0 {
		nightActs = []string{"准备睡觉"}
	}

	sm.Slots = []TimeSlot{
		{Range: "06:00-09:00", Activities: morningActs, Weights: equalWeights(len(morningActs))},
		{Range: "09:00-12:00", Activities: noonActs, Weights: equalWeights(len(noonActs))},
		{Range: "12:00-14:00", Activities: []string{"午餐", "午休"}, Weights: []float64{0.6, 0.4}},
		{Range: "14:00-18:00", Activities: afternoonActs, Weights: equalWeights(len(afternoonActs))},
		{Range: "18:00-22:00", Activities: eveningActs, Weights: equalWeights(len(eveningActs))},
		{Range: "22:00-06:00", Activities: nightActs, Weights: equalWeights(len(nightActs))},
	}

	return sm
}

func equalWeights(n int) []float64 {
	if n == 0 {
		return nil
	}
	w := 1.0 / float64(n)
	weights := make([]float64, n)
	for i := range weights {
		weights[i] = w
	}
	return weights
}
