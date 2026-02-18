package persona

import (
	"fmt"
	"strings"
	"unicode/utf8"

)

// ViolationType classifies the type of constraint 
type ViolationType string

const (
	ViolationEndsWithQuestion   ViolationType = "ends_with_question"
	ViolationExcessiveQuestions ViolationType = "excessive_questions"
	ViolationNoShareFirst      ViolationType = "no_share_first"
)

// ViolationResult is the outcome of a single violation check.
type ViolationResult struct {
	Violated bool
	Type     ViolationType
	Detail   string
	Severity string // "hard" = triggers retry, "soft" = log only
}

// DetectViolations checks the LLM output against style constraints.
// v1: 2 hard detections (question ending, excessive questions) + 1 soft (share_first).
func DetectViolations(output string, constraints StyleConstraints) []ViolationResult {
	var results []ViolationResult
	trimmed := strings.TrimSpace(output)

	// Hard detection 1: ends with question mark (accuracy >95%)
	if constraints.AvoidEndWithQuestionMark && len(trimmed) > 0 {
		lastRune, _ := utf8.DecodeLastRuneInString(trimmed)
		if lastRune == '？' || lastRune == '?' {
			results = append(results, ViolationResult{
				Violated: true,
				Type:     ViolationEndsWithQuestion,
				Detail:   "回复以问号结尾",
				Severity: "hard",
			})
		}
	}

	// Hard detection 2: too many questions (accuracy >90%)
	qCount := countQuestions(trimmed)
	if qCount > constraints.MaxQuestionsThisTurn {
		results = append(results, ViolationResult{
			Violated: true,
			Type:     ViolationExcessiveQuestions,
			Detail:   fmt.Sprintf("问题数 %d > 限制 %d", qCount, constraints.MaxQuestionsThisTurn),
			Severity: "hard",
		})
	}

	// Soft detection: share_first (accuracy ~60%, log only, no retry)
	if constraints.MustShareFirst && !weakShareCheck(trimmed) {
		results = append(results, ViolationResult{
			Violated: true,
			Type:     ViolationNoShareFirst,
			Detail:   "未先分享自我状态",
			Severity: "soft",
		})
	}

	return results
}

// FilterHard returns only hard-severity violations.
func FilterHard(violations []ViolationResult) []ViolationResult {
	var hard []ViolationResult
	for _, v := range violations {
		if v.Severity == "hard" {
			hard = append(hard, v)
		}
	}
	return hard
}

// countQuestions counts the number of question marks in the text.
func countQuestions(text string) int {
	count := 0
	for _, r := range text {
		if r == '？' || r == '?' {
			count++
		}
	}
	return count
}

// weakShareCheck is a conservative heuristic for share_first detection.
// Checks if the first sentence contains first-person pronoun + activity/state word.
func weakShareCheck(output string) bool {
	firstSentence := extractFirstSentence(output)
	if firstSentence == "" {
		return false
	}

	firstPersonWords := []string{"我", "我的", "我在", "我刚", "我今天"}
	activityWords := []string{"刚", "在", "正在", "今天", "觉得", "感觉", "想", "看了", "听了", "做了", "去了", "吃了"}

	hasFirstPerson := containsAny(firstSentence, firstPersonWords)
	hasActivity := containsAny(firstSentence, activityWords)

	return hasFirstPerson && hasActivity
}

func extractFirstSentence(text string) string {
	delimiters := []rune{'。', '！', '？', '!', '?', '\n'}
	runes := []rune(text)
	for i, r := range runes {
		for _, d := range delimiters {
			if r == d {
				return string(runes[:i+1])
			}
		}
	}
	// No delimiter found, take first 50 chars
	if len(runes) > 50 {
		return string(runes[:50])
	}
	return text
}

func containsAny(text string, words []string) bool {
	for _, w := range words {
		if strings.Contains(text, w) {
			return true
		}
	}
	return false
}
