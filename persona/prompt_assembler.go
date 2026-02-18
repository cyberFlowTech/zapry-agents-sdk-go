package persona

import (
	"fmt"
	"strings"

)

// AssemblePrompt compiles a PersonaSpec into a system prompt string.
// Structure: [Role Core] + [Personality] + [Thinking Style] + [Life Details] + [Boundaries] + [Chat Rules]
func AssemblePrompt(spec *PersonaSpec, tpl *Template, thinkingStyle string) string {
	var sections []string

	// [Role Core]
	roleCore := buildRoleCore(spec)
	sections = append(sections, roleCore)

	// [Personality Portrait]
	personalitySection := buildPersonality(spec)
	if personalitySection != "" {
		sections = append(sections, personalitySection)
	}

	// [Thinking Style] (from MBTI)
	if thinkingStyle != "" {
		sections = append(sections, "## 思维方式\n"+thinkingStyle)
	}

	// [Life Details] from signature_details
	lifeDetails := buildLifeDetails(spec)
	if lifeDetails != "" {
		sections = append(sections, lifeDetails)
	}

	// [Boundaries]
	if len(spec.Boundaries) > 0 {
		boundaries := "## 绝对规则\n"
		for _, b := range spec.Boundaries {
			boundaries += fmt.Sprintf("- %s\n", b)
		}
		sections = append(sections, boundaries)
	}

	// [Chat Rules from template]
	sections = append(sections, tpl.BaseSystemRules)

	prompt := strings.Join(sections, "\n\n")

	// Budget control: trim if over limit (default 1500 chars)
	maxChars := 1500
	runes := []rune(prompt)
	if len(runes) > maxChars {
		prompt = buildTrimmedPrompt(spec, tpl, thinkingStyle, maxChars)
	}

	return prompt
}

func buildRoleCore(spec *PersonaSpec) string {
	parts := []string{}
	parts = append(parts, fmt.Sprintf("你是%s", spec.Name))
	if spec.Age > 0 {
		parts = append(parts, fmt.Sprintf("，%d岁", spec.Age))
	}
	if spec.Profession != "" {
		parts = append(parts, fmt.Sprintf("的%s", spec.Profession))
	}
	return strings.Join(parts, "") + "。"
}

func buildPersonality(spec *PersonaSpec) string {
	if len(spec.Traits) == 0 {
		return ""
	}

	result := "## 性格画像\n"
	result += fmt.Sprintf("性格：%s。", strings.Join(spec.Traits, "、"))

	toneDesc := map[string]string{
		"warm":  "语气温暖亲切",
		"calm":  "语气平静沉稳",
		"sharp": "语气直接犀利",
	}
	if desc, ok := toneDesc[spec.Tone]; ok {
		result += desc + "。"
	}

	if len(spec.Hobbies) > 0 {
		result += fmt.Sprintf("\n喜欢：%s。", strings.Join(spec.Hobbies, "、"))
	}

	return result
}

func buildLifeDetails(spec *PersonaSpec) string {
	if len(spec.SignatureDetails) == 0 {
		return ""
	}

	details := "## 生活细节\n"

	if petName, ok := spec.SignatureDetails["pet_name"]; ok {
		details += fmt.Sprintf("- 你养了一只叫「%v」的宠物\n", petName)
	}
	if places, ok := spec.SignatureDetails["places"]; ok {
		if placeList, ok := places.([]any); ok {
			strs := make([]string, 0, len(placeList))
			for _, p := range placeList {
				strs = append(strs, fmt.Sprintf("%v", p))
			}
			details += fmt.Sprintf("- 你常去的地方：%s\n", strings.Join(strs, "、"))
		}
	}
	if music, ok := spec.SignatureDetails["music"]; ok {
		if musicList, ok := music.([]any); ok {
			strs := make([]string, 0, len(musicList))
			for _, m := range musicList {
				strs = append(strs, fmt.Sprintf("%v", m))
			}
			details += fmt.Sprintf("- 你喜欢听的音乐：%s\n", strings.Join(strs, "、"))
		}
	}
	if living, ok := spec.SignatureDetails["living"]; ok {
		details += fmt.Sprintf("- 你住在%v\n", living)
	}

	return details
}

func buildTrimmedPrompt(spec *PersonaSpec, tpl *Template, thinkingStyle string, maxChars int) string {
	// Build without life details first (keep core + personality + thinking + boundaries + rules)
	var sections []string
	sections = append(sections, buildRoleCore(spec))
	if p := buildPersonality(spec); p != "" {
		sections = append(sections, p)
	}
	if thinkingStyle != "" {
		sections = append(sections, "## 思维方式\n"+thinkingStyle)
	}
	if len(spec.Boundaries) > 0 {
		boundaries := "## 绝对规则\n"
		for _, b := range spec.Boundaries {
			boundaries += fmt.Sprintf("- %s\n", b)
		}
		sections = append(sections, boundaries)
	}
	sections = append(sections, tpl.BaseSystemRules)

	prompt := strings.Join(sections, "\n\n")
	runes := []rune(prompt)
	if len(runes) > maxChars {
		return string(runes[:maxChars])
	}
	return prompt
}
