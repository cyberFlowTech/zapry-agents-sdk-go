package agentsdk

import (
	"testing"
)

// ══════════════════════════════════════════════
// FeedbackDetector tests
// ══════════════════════════════════════════════

func TestFeedbackDetector_DetectConcise(t *testing.T) {
	d := NewFeedbackDetector(nil, 50, nil)
	result := d.Detect("太长了", nil)
	if !result.Matched {
		t.Fatal("should match concise feedback")
	}
	if result.Changes["style"] != "concise" {
		t.Fatalf("expected style=concise, got %s", result.Changes["style"])
	}
	if result.Triggers["style"] != "太长了" {
		t.Fatalf("expected trigger=太长了, got %s", result.Triggers["style"])
	}
}

func TestFeedbackDetector_DetectDetailed(t *testing.T) {
	d := NewFeedbackDetector(nil, 50, nil)
	result := d.Detect("详细说说", nil)
	if !result.Matched {
		t.Fatal("should match detailed feedback")
	}
	if result.Changes["style"] != "detailed" {
		t.Fatalf("expected style=detailed, got %s", result.Changes["style"])
	}
}

func TestFeedbackDetector_DetectCasualTone(t *testing.T) {
	d := NewFeedbackDetector(nil, 50, nil)
	result := d.Detect("说人话", nil)
	if !result.Matched {
		t.Fatal("should match casual tone")
	}
	if result.Changes["tone"] != "casual" {
		t.Fatalf("expected tone=casual, got %s", result.Changes["tone"])
	}
}

func TestFeedbackDetector_DetectFormalTone(t *testing.T) {
	d := NewFeedbackDetector(nil, 50, nil)
	result := d.Detect("专业一些", nil)
	if !result.Matched {
		t.Fatal("should match formal tone")
	}
	if result.Changes["tone"] != "formal" {
		t.Fatalf("expected tone=formal, got %s", result.Changes["tone"])
	}
}

func TestFeedbackDetector_NoMatch(t *testing.T) {
	d := NewFeedbackDetector(nil, 50, nil)
	result := d.Detect("今天天气真好", nil)
	if result.Matched {
		t.Fatal("should not match")
	}
}

func TestFeedbackDetector_LongMessageSkipped(t *testing.T) {
	d := NewFeedbackDetector(nil, 10, nil)
	result := d.Detect("这是一条超过十个字符的消息太长了应该跳过", nil)
	if result.Matched {
		t.Fatal("long message should be skipped")
	}
}

func TestFeedbackDetector_EmptyMessage(t *testing.T) {
	d := NewFeedbackDetector(nil, 50, nil)
	result := d.Detect("", nil)
	if result.Matched {
		t.Fatal("empty message should not match")
	}
}

func TestFeedbackDetector_WhitespaceMessage(t *testing.T) {
	d := NewFeedbackDetector(nil, 50, nil)
	result := d.Detect("   ", nil)
	if result.Matched {
		t.Fatal("whitespace message should not match")
	}
}

func TestFeedbackDetector_DedupSameValue(t *testing.T) {
	d := NewFeedbackDetector(nil, 50, nil)
	result := d.Detect("太长了", map[string]string{"style": "concise"})
	if result.Matched {
		t.Fatal("should not match when value is already the same")
	}
}

func TestFeedbackDetector_DedupDifferentValue(t *testing.T) {
	d := NewFeedbackDetector(nil, 50, nil)
	result := d.Detect("详细说说", map[string]string{"style": "concise"})
	if !result.Matched {
		t.Fatal("should match when value changes")
	}
	if result.Changes["style"] != "detailed" {
		t.Fatalf("expected style=detailed, got %s", result.Changes["style"])
	}
}

func TestFeedbackDetector_CustomPatterns(t *testing.T) {
	patterns := map[string]map[string][]string{
		"language": {
			"english": {"speak english", "in english"},
			"chinese": {"说中文", "用中文"},
		},
	}
	d := NewFeedbackDetector(patterns, 50, nil)

	result := d.Detect("speak english", nil)
	if !result.Matched {
		t.Fatal("should match custom pattern")
	}
	if result.Changes["language"] != "english" {
		t.Fatalf("expected language=english, got %s", result.Changes["language"])
	}

	// Default patterns should NOT work
	result2 := d.Detect("太长了", nil)
	if result2.Matched {
		t.Fatal("default pattern should not work with custom patterns")
	}
}

func TestFeedbackDetector_AddPattern(t *testing.T) {
	d := NewFeedbackDetector(nil, 50, nil)
	d.AddPattern("mood", "positive", []string{"开心", "好心情"})
	result := d.Detect("开心", nil)
	if !result.Matched {
		t.Fatal("should match added pattern")
	}
	if result.Changes["mood"] != "positive" {
		t.Fatalf("expected mood=positive, got %s", result.Changes["mood"])
	}
}

func TestFeedbackDetector_SetPatternsReplaces(t *testing.T) {
	d := NewFeedbackDetector(nil, 50, nil)
	d.SetPatterns(map[string]map[string][]string{
		"custom": {"val": {"触发词"}},
	})

	// Old patterns should not work
	r1 := d.Detect("太长了", nil)
	if r1.Matched {
		t.Fatal("old pattern should not work after SetPatterns")
	}

	// New pattern should work
	r2 := d.Detect("触发词", nil)
	if !r2.Matched {
		t.Fatal("new pattern should work after SetPatterns")
	}
}

func TestFeedbackDetector_DetectAndAdapt(t *testing.T) {
	d := NewFeedbackDetector(nil, 50, nil)
	prefs := map[string]string{"style": "balanced"}

	result := d.DetectAndAdapt("u1", "太长了", prefs)
	if !result.Matched {
		t.Fatal("should match")
	}
	if prefs["style"] != "concise" {
		t.Fatalf("expected prefs[style]=concise, got %s", prefs["style"])
	}
	if _, ok := prefs["updated_at"]; !ok {
		t.Fatal("should set updated_at")
	}
}

func TestFeedbackDetector_DetectAndAdaptNoMatch(t *testing.T) {
	d := NewFeedbackDetector(nil, 50, nil)
	prefs := map[string]string{"style": "balanced"}

	result := d.DetectAndAdapt("u1", "今天天气真好", prefs)
	if result.Matched {
		t.Fatal("should not match")
	}
	if prefs["style"] != "balanced" {
		t.Fatal("prefs should not change")
	}
	if _, ok := prefs["updated_at"]; ok {
		t.Fatal("should not set updated_at when no match")
	}
}

func TestFeedbackDetector_OnChangeCallback(t *testing.T) {
	var callbackUserID string
	var callbackChanges map[string]string

	onChange := func(userID string, changes map[string]string) {
		callbackUserID = userID
		callbackChanges = changes
	}

	d := NewFeedbackDetector(nil, 50, onChange)
	prefs := map[string]string{}
	d.DetectAndAdapt("u1", "太长了", prefs)

	if callbackUserID != "u1" {
		t.Fatalf("expected callback userID=u1, got %s", callbackUserID)
	}
	if callbackChanges["style"] != "concise" {
		t.Fatalf("expected callback style=concise, got %s", callbackChanges["style"])
	}
}

// ══════════════════════════════════════════════
// BuildPreferencePrompt tests
// ══════════════════════════════════════════════

func TestBuildPreferencePrompt_ConciseStyle(t *testing.T) {
	prompt := BuildPreferencePrompt(map[string]string{"style": "concise"}, nil, "")
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !containsChinese(prompt, "简洁") {
		t.Fatal("prompt should contain 简洁")
	}
}

func TestBuildPreferencePrompt_DetailedStyle(t *testing.T) {
	prompt := BuildPreferencePrompt(map[string]string{"style": "detailed"}, nil, "")
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !containsChinese(prompt, "详细") {
		t.Fatal("prompt should contain 详细")
	}
}

func TestBuildPreferencePrompt_CasualTone(t *testing.T) {
	prompt := BuildPreferencePrompt(map[string]string{"tone": "casual"}, nil, "")
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !containsChinese(prompt, "轻松") && !containsChinese(prompt, "口语") {
		t.Fatal("prompt should contain 轻松 or 口语")
	}
}

func TestBuildPreferencePrompt_NoMatch(t *testing.T) {
	prompt := BuildPreferencePrompt(map[string]string{"style": "balanced"}, nil, "")
	if prompt != "" {
		t.Fatalf("expected empty prompt, got %s", prompt)
	}
}

func TestBuildPreferencePrompt_Empty(t *testing.T) {
	prompt := BuildPreferencePrompt(map[string]string{}, nil, "")
	if prompt != "" {
		t.Fatalf("expected empty prompt, got %s", prompt)
	}
}

func TestBuildPreferencePrompt_SkipUpdatedAt(t *testing.T) {
	prompt := BuildPreferencePrompt(map[string]string{"updated_at": "2025-01-01"}, nil, "")
	if prompt != "" {
		t.Fatalf("expected empty prompt (updated_at should be skipped), got %s", prompt)
	}
}

func TestBuildPreferencePrompt_CustomMap(t *testing.T) {
	customMap := map[string]map[string]string{
		"mood": {
			"happy": "用户心情好，可以活泼一点。",
		},
	}
	prompt := BuildPreferencePrompt(map[string]string{"mood": "happy"}, customMap, "")
	if !containsChinese(prompt, "活泼") {
		t.Fatal("prompt should contain 活泼")
	}
}

func TestBuildPreferencePrompt_CustomHeader(t *testing.T) {
	prompt := BuildPreferencePrompt(map[string]string{"style": "concise"}, nil, "Style Preferences:")
	if len(prompt) < 18 || prompt[:18] != "Style Preferences:" {
		t.Fatalf("expected prompt to start with custom header, got %s", prompt)
	}
}

func TestBuildPreferencePrompt_DefaultHeader(t *testing.T) {
	prompt := BuildPreferencePrompt(map[string]string{"style": "concise"}, nil, "")
	expected := "回复风格偏好："
	if len(prompt) < len(expected) {
		t.Fatalf("prompt too short: %s", prompt)
	}
	header := prompt[:len(expected)]
	if header != expected {
		t.Fatalf("expected default header, got %s", header)
	}
}

// helper
func containsChinese(s, sub string) bool {
	return len(s) > 0 && len(sub) > 0 && contains(s, sub)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
