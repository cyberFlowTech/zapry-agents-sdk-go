package agentsdk

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ══════════════════════════════════════════════
// ConversationStateTracker tests
// ══════════════════════════════════════════════

func newTestSession() *MemorySession {
	store := NewInMemoryMemoryStore()
	return NewMemorySession("test_agent", "user_1", store)
}

func TestTrack_FirstConversation(t *testing.T) {
	tracker := NewConversationStateTracker("UTC")
	session := newTestSession()
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	state := tracker.Track(session, "hello", now)

	if !state.IsFirstConversation {
		t.Fatal("expected IsFirstConversation=true")
	}
	if state.DaysSinceLast != -1 {
		t.Fatalf("expected DaysSinceLast=-1, got %d", state.DaysSinceLast)
	}
	if state.TurnIndex != 1 {
		t.Fatalf("expected TurnIndex=1, got %d", state.TurnIndex)
	}
}

func TestTrack_DaysSinceLast(t *testing.T) {
	tracker := NewConversationStateTracker("UTC")
	session := newTestSession()
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	// Simulate a previous session 3 days ago
	tracker.TouchSession(session, now.AddDate(0, 0, -3))

	state := tracker.Track(session, "hello", now)

	if state.DaysSinceLast != 3 {
		t.Fatalf("expected DaysSinceLast=3, got %d", state.DaysSinceLast)
	}
	if state.IsFirstConversation {
		t.Fatal("should not be first conversation")
	}
}

func TestTrack_IsFollowUp(t *testing.T) {
	tracker := NewConversationStateTracker("UTC")
	session := newTestSession()
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	// First message
	state1 := tracker.Track(session, "hello", now)
	if state1.IsFollowUp {
		t.Fatal("first message should not be followup")
	}

	// Second message 30s later → followup
	state2 := tracker.Track(session, "more info", now.Add(30*time.Second))
	if !state2.IsFollowUp {
		t.Fatal("30s later should be followup")
	}

	// Third message 90s later → not followup
	state3 := tracker.Track(session, "new topic", now.Add(120*time.Second))
	if state3.IsFollowUp {
		t.Fatal("90s after last should not be followup")
	}
}

func TestTrack_TurnIndex(t *testing.T) {
	tracker := NewConversationStateTracker("UTC")
	session := newTestSession()
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	for i := 1; i <= 5; i++ {
		state := tracker.Track(session, "msg", now.Add(time.Duration(i)*time.Second))
		if state.TurnIndex != i {
			t.Fatalf("expected TurnIndex=%d, got %d", i, state.TurnIndex)
		}
	}
}

func TestTrack_TimeOfDay(t *testing.T) {
	tracker := NewConversationStateTracker("UTC")

	tests := []struct {
		hour     int
		expected string
	}{
		{7, "morning"}, {14, "afternoon"}, {20, "evening"}, {2, "late_night"},
	}

	for _, tt := range tests {
		session := newTestSession()
		now := time.Date(2025, 6, 15, tt.hour, 0, 0, 0, time.UTC)
		state := tracker.Track(session, "test", now)
		if state.TimeOfDay != tt.expected {
			t.Fatalf("hour %d: expected %s, got %s", tt.hour, tt.expected, state.TimeOfDay)
		}
	}
}

func TestTrack_LocalTime_WithTimezone(t *testing.T) {
	tracker := NewConversationStateTracker("Asia/Shanghai")
	session := newTestSession()
	now := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC) // 10:30 UTC = 18:30 Shanghai

	state := tracker.Track(session, "test", now)

	if !strings.Contains(state.LocalTime, "+08:00") && !strings.Contains(state.LocalTime, "CST") {
		t.Fatalf("expected Shanghai timezone in LocalTime, got %s", state.LocalTime)
	}
	// Should be evening in Shanghai
	if state.TimeOfDay != "evening" {
		t.Fatalf("18:30 Shanghai should be evening, got %s", state.TimeOfDay)
	}
}

func TestTrack_ToKV_Namespace(t *testing.T) {
	tracker := NewConversationStateTracker("UTC")
	session := newTestSession()
	state := tracker.Track(session, "test", time.Now())

	kv := state.ToKV()
	for key := range kv {
		if !strings.HasPrefix(key, "sdk.") {
			t.Fatalf("key %q should have sdk. prefix", key)
		}
	}
	expectedKeys := []string{
		"sdk.conversation.days_since_last", "sdk.conversation.total_sessions",
		"sdk.conversation.is_first", "sdk.session.turn_index",
		"sdk.user.is_followup", "sdk.user.msg_length",
		"sdk.runtime.time_of_day", "sdk.runtime.local_time",
	}
	for _, k := range expectedKeys {
		if _, ok := kv[k]; !ok {
			t.Fatalf("missing expected key: %s", k)
		}
	}
}

// ══════════════════════════════════════════════
// EmotionalToneDetector tests
// ══════════════════════════════════════════════

func TestDetect_Anxious_Chinese(t *testing.T) {
	d := NewEmotionalToneDetector()
	tone := d.Detect("快点给我看看结果", nil)
	if tone.Tone != "anxious" {
		t.Fatalf("expected anxious, got %s", tone.Tone)
	}
	if tone.Confidence < 0.3 {
		t.Fatalf("expected confidence >= 0.3, got %.2f", tone.Confidence)
	}
}

func TestDetect_Angry_StrongWord(t *testing.T) {
	d := NewEmotionalToneDetector()
	tone := d.Detect("什么破东西能不能正常点", nil)
	if tone.Tone != "angry" {
		t.Fatalf("expected angry, got %s", tone.Tone)
	}
	if tone.Confidence < 0.5 {
		t.Fatalf("expected confidence >= 0.5 for strong word, got %.2f", tone.Confidence)
	}
}

func TestDetect_Happy_LowWeight(t *testing.T) {
	d := NewEmotionalToneDetector()
	// Single "哈哈" has weight 0.3 — exactly at threshold, should detect
	tone := d.Detect("哈哈", nil)
	if tone.Tone != "happy" {
		t.Fatalf("expected happy, got %s (scores: %v)", tone.Tone, tone.Scores)
	}
}

func TestDetect_Happy_MultiHit(t *testing.T) {
	d := NewEmotionalToneDetector()
	tone := d.Detect("太好了哈哈真棒", nil)
	if tone.Tone != "happy" {
		t.Fatalf("expected happy, got %s", tone.Tone)
	}
	if tone.Confidence < 0.6 {
		t.Fatalf("expected confidence >= 0.6 for multi-hit, got %.2f", tone.Confidence)
	}
}

func TestDetect_English(t *testing.T) {
	d := NewEmotionalToneDetector()
	tone := d.Detect("I need this ASAP please hurry", nil)
	if tone.Tone != "anxious" {
		t.Fatalf("expected anxious for English, got %s", tone.Tone)
	}
}

func TestDetect_FollowUpBoost(t *testing.T) {
	d := NewEmotionalToneDetector()
	state := &ConversationState{IsFollowUp: true, UserMsgLength: "short"}
	tone := d.Detect("快", state) // keyword "快" partial match in "快点"? No, need exact. Let's use "急"
	tone2 := d.Detect("急", state)
	// "急" matches keyword, +0.4, plus followup boost +0.2 = 0.6
	if tone2.Scores["anxious"] < 0.5 {
		t.Fatalf("expected anxious score boosted, got %.2f", tone2.Scores["anxious"])
	}
	_ = tone
}

func TestDetect_Neutral_NoOutput(t *testing.T) {
	d := NewEmotionalToneDetector()
	tone := d.Detect("今天天气怎么样", nil)
	if tone.FormatForPrompt() != "" {
		t.Fatal("neutral should produce empty prompt")
	}
}

// ══════════════════════════════════════════════
// ResponseStyleController tests
// ══════════════════════════════════════════════

func TestPostProcess_TooLong_NaturalEnding(t *testing.T) {
	ctrl := NewResponseStyleController(StyleConfig{
		MaxLength:   30,
		MinPreserve: 10,
	})

	// ~38 rune text with sentence boundaries, MaxLength=30 → will truncate
	long := "第一句话到这里结束。第二句话继续说下去。第三句话还在延伸。第四句话也很长呢。"
	result, changed, violations := ctrl.PostProcess(long)

	if !changed {
		t.Fatal("expected change due to truncation")
	}
	// Should end with a natural ending, not "…"
	foundNatural := false
	for _, ending := range naturalEndings {
		if strings.HasSuffix(result, ending) {
			foundNatural = true
			break
		}
	}
	if !foundNatural {
		t.Fatalf("expected natural ending, got: %s", result)
	}
	if len(violations) == 0 || !strings.Contains(violations[0], "style.truncated") {
		t.Fatalf("expected truncated violation, got %v", violations)
	}
}

func TestPostProcess_MinPreserve_NoTruncate(t *testing.T) {
	ctrl := NewResponseStyleController(StyleConfig{
		MaxLength:   30,
		MinPreserve: 40,
	})

	short := "这是一段三十五个字的测试文本，不应该被截断因为还没到最小保留长度。" // ~30 runes
	result, changed, _ := ctrl.PostProcess(short)

	if changed {
		t.Fatalf("should not truncate below MinPreserve, got: %s", result)
	}
}

func TestPostProcess_ForbiddenPhrase_Removed(t *testing.T) {
	ctrl := NewResponseStyleController()
	input := "你好！作为一个AI，我来帮你解答。这是实际内容。"
	result, changed, violations := ctrl.PostProcess(input)

	if !changed {
		t.Fatal("expected change")
	}
	if strings.Contains(result, "作为一个AI") {
		t.Fatal("forbidden phrase should be removed")
	}
	if len(violations) == 0 {
		t.Fatal("expected violations")
	}
}

func TestPostProcess_EndQuestion_Fixed(t *testing.T) {
	ctrl := NewResponseStyleController(StyleConfig{EndStyle: "no_question"})
	result, changed, _ := ctrl.PostProcess("这样可以吗？")

	if !changed || !strings.HasSuffix(strings.TrimSpace(result), "。") {
		t.Fatalf("expected question mark replaced, got: %s", result)
	}
}

func TestPostProcess_Normal_NoChange(t *testing.T) {
	ctrl := NewResponseStyleController(StyleConfig{MaxLength: 500})
	input := "这是一段正常的回复。"
	result, changed, violations := ctrl.PostProcess(input)

	if changed {
		t.Fatalf("expected no change, got: %s", result)
	}
	if len(violations) > 0 {
		t.Fatalf("expected no violations, got %v", violations)
	}
}

func TestPostProcess_Warnings_Recorded(t *testing.T) {
	ctrl := NewResponseStyleController(StyleConfig{MaxLength: 20, MinPreserve: 10})
	input := "作为一个AI，我来帮你解答。这段话很长很长很长很长很长很长很长很长很长。"
	_, _, violations := ctrl.PostProcess(input)

	if len(violations) == 0 {
		t.Fatal("expected violations to be recorded")
	}
}

func TestBuildStylePrompt(t *testing.T) {
	ctrl := NewResponseStyleController(StyleConfig{PreferredLength: 150, EndStyle: "no_question"})
	prompt := ctrl.BuildStylePrompt()

	if !strings.Contains(prompt, "150") {
		t.Fatalf("expected preferred length in prompt, got: %s", prompt)
	}
	if !strings.Contains(prompt, "问句") {
		t.Fatalf("expected no_question hint, got: %s", prompt)
	}
}

// ══════════════════════════════════════════════
// ConversationOpener tests
// ══════════════════════════════════════════════

func TestOpener_FirstMeeting(t *testing.T) {
	g := NewOpenerGenerator()
	state := &ConversationState{IsFirstConversation: true, DaysSinceLast: -1}
	s := g.Generate(state, 0)
	if s.Situation != "first_meeting" {
		t.Fatalf("expected first_meeting, got %s", s.Situation)
	}
}

func TestOpener_LongAbsence(t *testing.T) {
	g := NewOpenerGenerator()
	state := &ConversationState{DaysSinceLast: 7, TotalSessions: 5}
	s := g.Generate(state, 0)
	if s.Situation != "long_absence" {
		t.Fatalf("expected long_absence, got %s", s.Situation)
	}
}

func TestOpener_FollowUp(t *testing.T) {
	g := NewOpenerGenerator()
	state := &ConversationState{IsFollowUp: true, DaysSinceLast: 0}
	s := g.Generate(state, 0)
	if s.Situation != "followup" {
		t.Fatalf("expected followup, got %s", s.Situation)
	}
}

func TestOpener_LateNight(t *testing.T) {
	g := NewOpenerGenerator()
	state := &ConversationState{TimeOfDay: "late_night", DaysSinceLast: 0}
	s := g.Generate(state, 0)
	if s.Situation != "late_night" {
		t.Fatalf("expected late_night, got %s", s.Situation)
	}
}

func TestOpener_FrequencyLimit(t *testing.T) {
	g := NewOpenerGenerator(OpenerConfig{MaxMentionsPerSession: 1})
	state := &ConversationState{IsFirstConversation: true, DaysSinceLast: -1}

	s1 := g.Generate(state, 0)
	if s1.Situation != "first_meeting" {
		t.Fatal("first call should generate opener")
	}

	s2 := g.Generate(state, 1) // already used 1
	if s2.Situation != "normal" {
		t.Fatalf("second call should be normal (frequency limit), got %s", s2.Situation)
	}
	if s2.Hint != "" {
		t.Fatal("no hint when frequency limited")
	}
}

func TestOpener_Normal(t *testing.T) {
	g := NewOpenerGenerator()
	state := &ConversationState{DaysSinceLast: 0, TimeOfDay: "afternoon"}
	s := g.Generate(state, 0)
	if s.Situation != "normal" {
		t.Fatalf("expected normal, got %s", s.Situation)
	}
}

// ══════════════════════════════════════════════
// ContextCompressor tests
// ══════════════════════════════════════════════

func makeHistory(n int) []map[string]interface{} {
	msgs := make([]map[string]interface{}, n)
	for i := 0; i < n; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs[i] = map[string]interface{}{
			"role":    role,
			"content": fmt.Sprintf("Message %d with some content that takes up tokens.", i),
		}
	}
	return msgs
}

func TestCompress_BelowThreshold_NoChange(t *testing.T) {
	called := false
	fn := func(msgs []map[string]interface{}) (string, error) {
		called = true
		return "summary", nil
	}
	comp := NewContextCompressor(fn, CompressorConfig{TokenThreshold: 99999})
	wm := NewWorkingMemory()

	history := makeHistory(5)
	result, err := comp.Compress(history, wm)
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("should not call summarize below threshold")
	}
	if len(result) != 5 {
		t.Fatalf("expected 5 messages unchanged, got %d", len(result))
	}
}

func TestCompress_AboveThreshold_Summarized(t *testing.T) {
	fn := func(msgs []map[string]interface{}) (string, error) {
		return "This is the summary.", nil
	}
	comp := NewContextCompressor(fn, CompressorConfig{
		WindowSize:     2,
		TokenThreshold: 1, // force compression
		SummaryVersion: "v1",
	})
	wm := NewWorkingMemory()

	history := makeHistory(10)
	result, err := comp.Compress(history, wm)
	if err != nil {
		t.Fatal(err)
	}
	// Should be: 1 summary + 2 recent = 3
	if len(result) != 3 {
		t.Fatalf("expected 3 messages (summary + 2 recent), got %d", len(result))
	}
	if result[0]["role"] != "system" {
		t.Fatal("first message should be system (summary)")
	}
}

func TestCompress_SummaryHasTag(t *testing.T) {
	fn := func(msgs []map[string]interface{}) (string, error) {
		return "Summary content.", nil
	}
	comp := NewContextCompressor(fn, CompressorConfig{
		WindowSize:     2,
		TokenThreshold: 1,
		SummaryVersion: "v1",
	})
	wm := NewWorkingMemory()

	result, _ := comp.Compress(makeHistory(10), wm)
	content := result[0]["content"].(string)
	if !strings.HasPrefix(content, "[sdk.summary:v1]") {
		t.Fatalf("expected sdk.summary tag, got: %s", content)
	}
}

func TestCompress_CacheHit(t *testing.T) {
	callCount := 0
	fn := func(msgs []map[string]interface{}) (string, error) {
		callCount++
		return "cached summary", nil
	}
	comp := NewContextCompressor(fn, CompressorConfig{
		WindowSize:     2,
		TokenThreshold: 1,
		SummaryVersion: "v1",
	})
	wm := NewWorkingMemory()

	comp.Compress(makeHistory(10), wm) // first call
	comp.Compress(makeHistory(10), wm) // second call — should use cache

	if callCount != 1 {
		t.Fatalf("expected 1 summarize call (cached), got %d", callCount)
	}
}

func TestCompress_VersionChange_CacheInvalidated(t *testing.T) {
	callCount := 0
	fn := func(msgs []map[string]interface{}) (string, error) {
		callCount++
		return "summary", nil
	}
	wm := NewWorkingMemory()

	comp1 := NewContextCompressor(fn, CompressorConfig{WindowSize: 2, TokenThreshold: 1, SummaryVersion: "v1"})
	comp1.Compress(makeHistory(10), wm) // call 1

	comp2 := NewContextCompressor(fn, CompressorConfig{WindowSize: 2, TokenThreshold: 1, SummaryVersion: "v2"})
	comp2.Compress(makeHistory(10), wm) // call 2 — cache invalid

	if callCount != 2 {
		t.Fatalf("expected 2 calls (cache invalidated by version), got %d", callCount)
	}
}

func TestCompress_CustomEstimator(t *testing.T) {
	fn := func(msgs []map[string]interface{}) (string, error) {
		return "summary", nil
	}
	// Custom estimator that always returns very high token count
	customEstimator := func(history []map[string]interface{}) int {
		return 99999
	}
	comp := NewContextCompressor(fn, CompressorConfig{
		WindowSize:       2,
		TokenThreshold:   5000,
		EstimateTokensFn: customEstimator,
	})
	wm := NewWorkingMemory()

	result, _ := comp.Compress(makeHistory(5), wm)
	// Should compress even with few messages because custom estimator says tokens are high
	if len(result) != 3 { // 1 summary + 2 recent
		t.Fatalf("expected compression with custom estimator, got %d messages", len(result))
	}
}

// ══════════════════════════════════════════════
// NaturalConversation integration tests
// ══════════════════════════════════════════════

func TestNaturalConversation_Enhance_DefaultConfig(t *testing.T) {
	nc := NewNaturalConversation(DefaultNaturalConversationConfig())
	session := newTestSession()
	now := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	fragments, history := nc.Enhance(session, "你好呀", nil, now)

	if fragments.Text() == "" {
		t.Fatal("expected non-empty extra_context")
	}
	if len(fragments.KV) == 0 {
		t.Fatal("expected KV to be populated")
	}
	if len(fragments.Warnings) == 0 {
		t.Fatal("expected warnings to be recorded")
	}
	_ = history
}

func TestNaturalConversation_PostProcess(t *testing.T) {
	nc := NewNaturalConversation(DefaultNaturalConversationConfig())
	output := "这是回复。希望对你有帮助？"
	result, changed := nc.PostProcess(output)

	if !changed {
		t.Fatal("expected postprocess to make changes (forbidden phrase + question mark)")
	}
	if strings.Contains(result, "希望对你有帮助") {
		t.Fatal("forbidden phrase should be removed")
	}
}

func TestNaturalConversation_WrapLoop(t *testing.T) {
	nc := NewNaturalConversation(DefaultNaturalConversationConfig())

	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		return &LLMMessage{Content: "Hello there!"}, nil
	}
	loop := NewAgentLoop(llm, NewToolRegistry(), "", 10, nil)
	naturalLoop := nc.WrapLoop(loop)

	session := newTestSession()
	result := naturalLoop.Run(session, "hi", nil)

	if result.StoppedReason != "completed" {
		t.Fatalf("expected completed, got %s", result.StoppedReason)
	}
	if result.FinalOutput == "" {
		t.Fatal("expected non-empty output")
	}
	if naturalLoop.LastFragments() == nil {
		t.Fatal("expected LastFragments to be available")
	}
}

func TestNaturalConversation_WrapLoop_WithContext(t *testing.T) {
	nc := NewNaturalConversation(DefaultNaturalConversationConfig())

	llm := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		return &LLMMessage{Content: "response"}, nil
	}
	loop := NewAgentLoop(llm, NewToolRegistry(), "", 10, nil)
	naturalLoop := nc.WrapLoop(loop)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	session := newTestSession()
	result := naturalLoop.RunContext(ctx, session, "hi", nil)

	if result.StoppedReason != "cancelled" {
		t.Fatalf("expected cancelled, got %s", result.StoppedReason)
	}
}

func TestNaturalConversation_DefaultConfig_RecommendedOnly(t *testing.T) {
	config := DefaultNaturalConversationConfig()

	if !config.StateTracking {
		t.Fatal("StateTracking should be ON by default")
	}
	if !config.EmotionDetection {
		t.Fatal("EmotionDetection should be ON by default")
	}
	if !config.StylePostProcess {
		t.Fatal("StylePostProcess should be ON by default")
	}
	if config.OpenerGeneration {
		t.Fatal("OpenerGeneration should be OFF by default")
	}
	if config.ContextCompress {
		t.Fatal("ContextCompress should be OFF by default")
	}
	if config.StyleRetry {
		t.Fatal("StyleRetry should be OFF by default")
	}
}
