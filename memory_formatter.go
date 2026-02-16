package agentsdk

import (
	"fmt"
	"strings"
)

// FormatMemoryForPrompt formats memory layers into text for LLM system prompt injection.
// Returns empty string if no meaningful content.
func FormatMemoryForPrompt(longTerm map[string]interface{}, working map[string]interface{}, template string) string {
	var parts []string

	ltText := formatLongTerm(longTerm)
	if ltText != "" {
		parts = append(parts, ltText)
	}

	if len(working) > 0 {
		var items []string
		for k, v := range working {
			if v != nil && fmt.Sprintf("%v", v) != "" {
				items = append(items, fmt.Sprintf("- %s: %v", k, v))
			}
		}
		if len(items) > 0 {
			parts = append(parts, "当前会话上下文：\n"+strings.Join(items, "\n"))
		}
	}

	if len(parts) == 0 {
		return ""
	}

	combined := strings.Join(parts, "\n\n")
	if template != "" {
		return strings.Replace(template, "{long_term_text}", combined, 1)
	}
	return "以下是该用户的个人信息（不是你自己的信息）。\n当用户问关于自己的问题时，必须根据以下档案回答：\n\n" + combined
}

func formatLongTerm(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	var lines []string

	// Basic info
	if basic, ok := m["basic_info"].(map[string]interface{}); ok && len(basic) > 0 {
		labels := map[string]string{
			"age": "年龄", "gender": "性别", "location": "位置",
			"occupation": "职业", "school": "学校", "major": "专业",
			"nickname": "昵称", "birthday": "生日",
		}
		var infoLines []string
		for field, label := range labels {
			if v, ok := basic[field]; ok && v != nil && fmt.Sprintf("%v", v) != "" {
				infoLines = append(infoLines, fmt.Sprintf("  - %s: %v", label, v))
			}
		}
		if len(infoLines) > 0 {
			lines = append(lines, "用户基本信息：")
			lines = append(lines, infoLines...)
		}
	}

	// Personality
	if p, ok := m["personality"].(map[string]interface{}); ok {
		if traits := toStringSlice(p["traits"]); len(traits) > 0 {
			lines = append(lines, "性格特点: "+strings.Join(traits, ", "))
		}
		if values := toStringSlice(p["values"]); len(values) > 0 {
			lines = append(lines, "价值观: "+strings.Join(values, ", "))
		}
	}

	// Life context
	if life, ok := m["life_context"].(map[string]interface{}); ok {
		if items := toStringSlice(life["concerns"]); len(items) > 0 {
			lines = append(lines, "当前困扰: "+strings.Join(items, ", "))
		}
		if items := toStringSlice(life["goals"]); len(items) > 0 {
			lines = append(lines, "目标: "+strings.Join(items, ", "))
		}
		if items := toStringSlice(life["recent_events"]); len(items) > 0 {
			lines = append(lines, "近期事件: "+strings.Join(items, ", "))
		}
	}

	// Interests
	if items := toStringSlice(m["interests"]); len(items) > 0 {
		lines = append(lines, "兴趣爱好: "+strings.Join(items, ", "))
	}

	// Summary
	if s, ok := m["summary"].(string); ok && s != "" {
		lines = append(lines, "用户特点: "+s)
	}

	// Count
	if meta, ok := m["meta"].(map[string]interface{}); ok {
		if count, ok := meta["conversation_count"].(float64); ok && count > 0 {
			lines = append(lines, fmt.Sprintf("（已对话 %.0f 次）", count))
		}
	}

	return strings.Join(lines, "\n")
}

func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s := fmt.Sprintf("%v", item); s != "" {
			result = append(result, s)
		}
	}
	return result
}
