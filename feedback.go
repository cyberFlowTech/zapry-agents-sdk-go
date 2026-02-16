package agentsdk

import (
	"log"
	"strings"
	"time"
)

// ──────────────────────────────────────────────
// Default Chinese feedback patterns
// ──────────────────────────────────────────────

// DefaultFeedbackPatterns provides Chinese keywords for feedback detection.
// Structure: pref_key -> pref_value -> []keywords
//
// Override via FeedbackDetector.SetPatterns() or AddPattern().
func DefaultFeedbackPatterns() map[string]map[string][]string {
	return map[string]map[string][]string{
		"style": {
			"concise":  {"太长了", "啰嗦", "简短点", "说重点", "太多了", "精简", "简洁"},
			"detailed": {"详细说说", "展开讲讲", "多说一些", "说详细点", "具体讲讲"},
		},
		"tone": {
			"casual": {"说人话", "白话", "通俗点", "别那么正式", "轻松一点"},
			"formal": {"专业一些", "正式一些", "文雅一些"},
		},
	}
}

// DefaultPreferencePrompts maps preference values to prompt text for AI injection.
func DefaultPreferencePrompts() map[string]map[string]string {
	return map[string]map[string]string{
		"style": {
			"concise":  "这位用户偏好简洁的回复，请控制在 100 字以内，直接说重点。",
			"detailed": "这位用户喜欢详细的解读，可以展开讲解，不用担心太长。",
		},
		"tone": {
			"casual": "这位用户喜欢轻松口语化的表达，少用正式或文言风格。",
			"formal": "这位用户喜欢专业正式的表达风格。",
		},
	}
}

// ──────────────────────────────────────────────
// FeedbackResult
// ──────────────────────────────────────────────

// FeedbackResult holds the detection result.
type FeedbackResult struct {
	// Matched indicates whether any feedback signal was detected.
	Matched bool
	// Changes contains preference changes: pref_key -> new_value.
	Changes map[string]string
	// Triggers contains matched keywords: pref_key -> keyword.
	Triggers map[string]string
}

// ──────────────────────────────────────────────
// OnChangeFn callback
// ──────────────────────────────────────────────

// OnChangeFn is called when preferences are updated.
type OnChangeFn func(userID string, changes map[string]string)

// ──────────────────────────────────────────────
// FeedbackDetector
// ──────────────────────────────────────────────

// FeedbackDetector detects user feedback signals from messages
// and maps them to preference adjustments.
//
// Usage:
//
//	detector := agentsdk.NewFeedbackDetector(nil, 50, nil)
//	result := detector.Detect("太长了，说重点", currentPrefs)
//	if result.Matched {
//	    for k, v := range result.Changes {
//	        prefs[k] = v
//	    }
//	}
type FeedbackDetector struct {
	patterns  map[string]map[string][]string
	maxLength int
	onChange  OnChangeFn
}

// NewFeedbackDetector creates a new detector.
//
// Parameters:
//   - patterns: custom keyword patterns (nil = use default Chinese patterns)
//   - maxLength: messages longer than this are skipped (0 = default 50)
//   - onChange: optional callback when preferences change
func NewFeedbackDetector(patterns map[string]map[string][]string, maxLength int, onChange OnChangeFn) *FeedbackDetector {
	if patterns == nil {
		patterns = DefaultFeedbackPatterns()
	}
	if maxLength <= 0 {
		maxLength = 50
	}
	return &FeedbackDetector{
		patterns:  patterns,
		maxLength: maxLength,
		onChange:  onChange,
	}
}

// SetPatterns replaces the entire keyword pattern map.
func (d *FeedbackDetector) SetPatterns(patterns map[string]map[string][]string) {
	d.patterns = patterns
}

// AddPattern appends keywords for a specific preference key/value.
//
// Example:
//
//	detector.AddPattern("language", "english", []string{"speak english", "in english"})
func (d *FeedbackDetector) AddPattern(prefKey, prefValue string, keywords []string) {
	if d.patterns[prefKey] == nil {
		d.patterns[prefKey] = make(map[string][]string)
	}
	d.patterns[prefKey][prefValue] = append(d.patterns[prefKey][prefValue], keywords...)
}

// Detect checks a message for feedback signals.
//
// Parameters:
//   - message: user message text
//   - currentPreferences: user's current preferences (for dedup; nil = no dedup)
//
// Returns FeedbackResult. Changes only includes values that differ from current.
func (d *FeedbackDetector) Detect(message string, currentPreferences map[string]string) FeedbackResult {
	result := FeedbackResult{
		Changes:  make(map[string]string),
		Triggers: make(map[string]string),
	}

	msg := strings.TrimSpace(message)
	if msg == "" || len([]rune(msg)) > d.maxLength {
		return result
	}

	current := currentPreferences
	if current == nil {
		current = make(map[string]string)
	}

	for prefKey, valueMap := range d.patterns {
		for prefValue, keywords := range valueMap {
			for _, kw := range keywords {
				if strings.Contains(msg, kw) {
					oldVal := current[prefKey]
					if oldVal != prefValue {
						result.Matched = true
						result.Changes[prefKey] = prefValue
						result.Triggers[prefKey] = kw
					}
					break
				}
			}
			if _, found := result.Changes[prefKey]; found {
				break
			}
		}
	}

	return result
}

// DetectAndAdapt detects feedback, updates the preferences map in-place,
// and invokes the onChange callback if set.
//
// Parameters:
//   - userID: user identifier
//   - message: user message text
//   - preferences: user preferences map (modified in place)
//
// Returns FeedbackResult.
func (d *FeedbackDetector) DetectAndAdapt(userID, message string, preferences map[string]string) FeedbackResult {
	result := d.Detect(message, preferences)
	if result.Matched {
		for k, v := range result.Changes {
			preferences[k] = v
		}
		preferences["updated_at"] = time.Now().Format(time.RFC3339)

		for prefKey, kw := range result.Triggers {
			log.Printf("[FeedbackDetector] Preference adapted | user=%s | %s -> %s | keyword=%s",
				userID, prefKey, result.Changes[prefKey], kw)
		}

		if d.onChange != nil {
			d.onChange(userID, result.Changes)
		}
	}
	return result
}

// ──────────────────────────────────────────────
// BuildPreferencePrompt
// ──────────────────────────────────────────────

// BuildPreferencePrompt generates a prompt string for AI system message injection
// based on user preferences.
//
// Parameters:
//   - preferences: user preferences {"style": "concise", "tone": "casual"}
//   - promptMap: custom mapping (nil = use default Chinese prompts)
//   - header: prompt header line (empty = "回复风格偏好：")
//
// Returns the assembled prompt string, or empty string if no matching preferences.
//
// Example:
//
//	prompt := agentsdk.BuildPreferencePrompt(prefs, nil, "")
//	if prompt != "" {
//	    messages = append(messages, SystemMessage{Content: prompt})
//	}
func BuildPreferencePrompt(preferences map[string]string, promptMap map[string]map[string]string, header string) string {
	if promptMap == nil {
		promptMap = DefaultPreferencePrompts()
	}
	if header == "" {
		header = "回复风格偏好："
	}

	var hints []string
	for prefKey, prefValue := range preferences {
		// Skip metadata
		if prefKey == "updated_at" {
			continue
		}
		if valuePrompts, ok := promptMap[prefKey]; ok {
			if text, ok := valuePrompts[prefValue]; ok {
				hints = append(hints, text)
			}
		}
	}

	if len(hints) == 0 {
		return ""
	}

	return header + "\n" + strings.Join(hints, "\n")
}
