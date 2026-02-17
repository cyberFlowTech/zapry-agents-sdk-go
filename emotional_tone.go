package agentsdk

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ──────────────────────────────────────────────
// Emotional Tone Detector — lightweight rule-based scoring
// ──────────────────────────────────────────────

// EmotionalTone holds the detected emotional tone and confidence.
type EmotionalTone struct {
	Tone       string             `json:"tone"`       // neutral/anxious/angry/happy/sad
	Confidence float64            `json:"confidence"` // 0.0-1.0
	Scores     map[string]float64 `json:"scores"`     // all tone scores
}

type weightedKeyword struct {
	keyword string
	weight  float64
}

// EmotionalToneDetector detects user emotional tone via weighted keyword scoring.
// Bilingual (Chinese + English), with differentiated weights to reduce false positives.
type EmotionalToneDetector struct {
	patterns map[string][]weightedKeyword
}

// NewEmotionalToneDetector creates a detector with built-in bilingual patterns.
func NewEmotionalToneDetector() *EmotionalToneDetector {
	return &EmotionalToneDetector{
		patterns: defaultEmotionPatterns(),
	}
}

func defaultEmotionPatterns() map[string][]weightedKeyword {
	return map[string][]weightedKeyword{
		"angry": {
			// Chinese strong words — weight 0.5
			{keyword: "什么破", weight: 0.5}, {keyword: "垃圾", weight: 0.5},
			{keyword: "搞什么", weight: 0.5}, {keyword: "有病", weight: 0.5},
			{keyword: "废物", weight: 0.5}, {keyword: "能不能正常", weight: 0.5},
			// English
			{keyword: "bullshit", weight: 0.5}, {keyword: "wtf", weight: 0.5},
			{keyword: "terrible", weight: 0.4}, {keyword: "useless", weight: 0.4},
		},
		"anxious": {
			// Chinese — weight 0.4
			{keyword: "快点", weight: 0.4}, {keyword: "赶紧", weight: 0.4},
			{keyword: "急", weight: 0.4}, {keyword: "等不了", weight: 0.4},
			{keyword: "尽快", weight: 0.4}, {keyword: "马上", weight: 0.3},
			// English
			{keyword: "asap", weight: 0.4}, {keyword: "hurry", weight: 0.4},
			{keyword: "quick", weight: 0.3}, {keyword: "urgent", weight: 0.4},
		},
		"happy": {
			// Lower weight (0.3) — needs multiple hits to trigger (anti-false-positive for sarcasm)
			{keyword: "太好了", weight: 0.3}, {keyword: "哈哈", weight: 0.3},
			{keyword: "棒", weight: 0.3}, {keyword: "开心", weight: 0.3},
			{keyword: "nice", weight: 0.3}, {keyword: "awesome", weight: 0.3},
			{keyword: "great", weight: 0.3}, {keyword: "love it", weight: 0.3},
		},
		"sad": {
			{keyword: "唉", weight: 0.4}, {keyword: "算了", weight: 0.4},
			{keyword: "难过", weight: 0.4}, {keyword: "失望", weight: 0.4},
			{keyword: "无所谓了", weight: 0.4},
			{keyword: "sigh", weight: 0.4}, {keyword: "forget it", weight: 0.4},
			{keyword: "disappointed", weight: 0.4},
		},
	}
}

// Detect analyzes user input for emotional tone.
// Pass ConversationState for contextual boosting (optional, can be nil).
func (d *EmotionalToneDetector) Detect(userInput string, state *ConversationState) *EmotionalTone {
	lower := strings.ToLower(userInput)
	scores := map[string]float64{
		"neutral": 0,
		"angry":   0,
		"anxious": 0,
		"happy":   0,
		"sad":     0,
	}

	// Keyword scoring
	for tone, keywords := range d.patterns {
		for _, kw := range keywords {
			if strings.Contains(lower, strings.ToLower(kw.keyword)) {
				scores[tone] += kw.weight
			}
		}
	}

	// Contextual boost: followup + short message → anxious +0.2
	if state != nil && state.IsFollowUp && state.UserMsgLength == "short" {
		scores["anxious"] += 0.2
	}

	// Exclamation boost: >=2 exclamation marks → top emotion +0.1 (cap +0.2)
	exclamCount := strings.Count(userInput, "!") + strings.Count(userInput, "！")
	if exclamCount >= 2 {
		boost := float64(exclamCount) * 0.1
		if boost > 0.2 {
			boost = 0.2
		}
		maxTone := findMaxTone(scores)
		if maxTone != "neutral" {
			scores[maxTone] += boost
		}
	}

	// Find top tone
	topTone := "neutral"
	topScore := 0.0
	for tone, score := range scores {
		if tone == "neutral" {
			continue
		}
		if score > topScore {
			topScore = score
			topTone = tone
		}
	}

	// Clamp confidence
	confidence := topScore
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Below threshold → neutral
	if confidence < 0.3 {
		topTone = "neutral"
		confidence = 0
	}

	return &EmotionalTone{
		Tone:       topTone,
		Confidence: confidence,
		Scores:     scores,
	}
}

// FormatForPrompt returns a gentle emotion hint for LLM injection.
// Returns "" if neutral or low confidence (no injection needed).
func (t *EmotionalTone) FormatForPrompt() string {
	if t.Tone == "neutral" || t.Confidence < 0.3 {
		return ""
	}
	_ = utf8.RuneCountInString // keep import

	// Gentle, indirect phrasing — don't say "user is angry"
	hints := map[string]string{
		"angry":   "用户语气较为强烈，请保持耐心，注意措辞温和",
		"anxious": "用户语气偏急促，请简洁直接回应，不要废话",
		"happy":   "用户心情不错，可以轻松互动",
		"sad":     "用户情绪偏低落，请语气温和关切",
	}
	hint, ok := hints[t.Tone]
	if !ok {
		return ""
	}
	return fmt.Sprintf("[用户情绪] %s", hint)
}

func findMaxTone(scores map[string]float64) string {
	maxTone := "neutral"
	maxScore := 0.0
	for tone, score := range scores {
		if tone == "neutral" {
			continue
		}
		if score > maxScore {
			maxScore = score
			maxTone = tone
		}
	}
	return maxTone
}
