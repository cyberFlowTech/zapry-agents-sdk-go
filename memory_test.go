package agentsdk

import (
	"encoding/json"
	"testing"
)

// ══════════════════════════════════════════════
// InMemoryMemoryStore
// ══════════════════════════════════════════════

func TestMemStore_KVGetSet(t *testing.T) {
	s := NewInMemoryMemoryStore()
	s.Set("ns", "k", "v")
	v, _ := s.Get("ns", "k")
	if v != "v" {
		t.Fatalf("expected v, got %s", v)
	}
	v2, _ := s.Get("ns", "missing")
	if v2 != "" {
		t.Fatal("expected empty string for missing key")
	}
}

func TestMemStore_KVDelete(t *testing.T) {
	s := NewInMemoryMemoryStore()
	s.Set("ns", "k", "v")
	s.Delete("ns", "k")
	v, _ := s.Get("ns", "k")
	if v != "" {
		t.Fatal("expected empty after delete")
	}
}

func TestMemStore_ListAppendGet(t *testing.T) {
	s := NewInMemoryMemoryStore()
	s.Append("ns", "l", "a")
	s.Append("ns", "l", "b")
	items, _ := s.GetList("ns", "l", 0, 0)
	if len(items) != 2 || items[0] != "a" || items[1] != "b" {
		t.Fatalf("expected [a b], got %v", items)
	}
}

func TestMemStore_ListTrim(t *testing.T) {
	s := NewInMemoryMemoryStore()
	for i := 0; i < 10; i++ {
		s.Append("ns", "l", string(rune('0'+i)))
	}
	s.TrimList("ns", "l", 3)
	items, _ := s.GetList("ns", "l", 0, 0)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

func TestMemStore_ListClear(t *testing.T) {
	s := NewInMemoryMemoryStore()
	s.Append("ns", "l", "x")
	s.ClearList("ns", "l")
	n, _ := s.ListLength("ns", "l")
	if n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

func TestMemStore_NamespaceIsolation(t *testing.T) {
	s := NewInMemoryMemoryStore()
	s.Set("a1:u1", "k", "v1")
	s.Set("a2:u1", "k", "v2")
	v1, _ := s.Get("a1:u1", "k")
	v2, _ := s.Get("a2:u1", "k")
	if v1 != "v1" || v2 != "v2" {
		t.Fatal("namespace isolation failed")
	}
}

// ══════════════════════════════════════════════
// WorkingMemory
// ══════════════════════════════════════════════

func TestWorkingMemory_GetSet(t *testing.T) {
	wm := NewWorkingMemory()
	wm.Set("intent", "tarot")
	if wm.Get("intent") != "tarot" {
		t.Fatal("expected tarot")
	}
	if wm.Len() != 1 {
		t.Fatal("expected len 1")
	}
}

func TestWorkingMemory_Clear(t *testing.T) {
	wm := NewWorkingMemory()
	wm.Set("a", 1)
	wm.Clear()
	if wm.Len() != 0 {
		t.Fatal("expected empty after clear")
	}
}

// ══════════════════════════════════════════════
// ShortTermMemory
// ══════════════════════════════════════════════

func TestSTM_AddAndGet(t *testing.T) {
	s := NewInMemoryMemoryStore()
	stm := NewShortTermMemory(s, "test:u1", 5)
	stm.AddMessage("user", "hello")
	stm.AddMessage("assistant", "hi")
	msgs, _ := stm.GetHistory(0)
	if len(msgs) != 2 || msgs[0].Role != "user" {
		t.Fatalf("unexpected history: %+v", msgs)
	}
}

func TestSTM_AutoTrim(t *testing.T) {
	s := NewInMemoryMemoryStore()
	stm := NewShortTermMemory(s, "test:u1", 5)
	for i := 0; i < 10; i++ {
		stm.AddMessage("user", "msg")
	}
	msgs, _ := stm.GetHistory(0)
	if len(msgs) != 5 {
		t.Fatalf("expected 5 after trim, got %d", len(msgs))
	}
}

func TestSTM_Clear(t *testing.T) {
	s := NewInMemoryMemoryStore()
	stm := NewShortTermMemory(s, "test:u1", 40)
	stm.AddMessage("user", "x")
	stm.Clear()
	n, _ := stm.Count()
	if n != 0 {
		t.Fatal("expected 0 after clear")
	}
}

func TestSTM_GetHistoryMaps(t *testing.T) {
	s := NewInMemoryMemoryStore()
	stm := NewShortTermMemory(s, "test:u1", 40)
	stm.AddMessage("user", "hi")
	maps, _ := stm.GetHistoryMaps(0)
	if maps[0]["role"] != "user" {
		t.Fatal("expected role=user")
	}
}

// ══════════════════════════════════════════════
// LongTermMemory
// ══════════════════════════════════════════════

func TestLTM_GetDefault(t *testing.T) {
	s := NewInMemoryMemoryStore()
	ltm := NewLongTermMemory(s, "test:u1", 0)
	data, _ := ltm.Get()
	if _, ok := data["basic_info"]; !ok {
		t.Fatal("expected default schema")
	}
}

func TestLTM_SaveAndGet(t *testing.T) {
	s := NewInMemoryMemoryStore()
	ltm := NewLongTermMemory(s, "test:u1", 0)
	ltm.Save(map[string]interface{}{"custom": "data", "meta": map[string]interface{}{}})
	data, _ := ltm.Get()
	if data["custom"] != "data" {
		t.Fatal("expected custom=data")
	}
}

func TestLTM_DeepMergeUpdate(t *testing.T) {
	s := NewInMemoryMemoryStore()
	ltm := NewLongTermMemory(s, "test:u1", 0)
	ltm.Save(map[string]interface{}{
		"basic_info": map[string]interface{}{"age": float64(25)},
		"interests":  []interface{}{"coding"},
		"meta":       map[string]interface{}{"conversation_count": float64(0)},
	})
	result, _ := ltm.Update(map[string]interface{}{
		"basic_info": map[string]interface{}{"location": "Shanghai"},
		"interests":  []interface{}{"music"},
	})
	bi := result["basic_info"].(map[string]interface{})
	if bi["age"] != float64(25) || bi["location"] != "Shanghai" {
		t.Fatalf("merge failed: %v", bi)
	}
	interests := result["interests"].([]interface{})
	if len(interests) != 2 {
		t.Fatalf("expected 2 interests, got %v", interests)
	}
}

func TestLTM_UpdateIncrementsCount(t *testing.T) {
	s := NewInMemoryMemoryStore()
	ltm := NewLongTermMemory(s, "test:u1", 0)
	ltm.Save(map[string]interface{}{"meta": map[string]interface{}{"conversation_count": float64(5)}})
	result, _ := ltm.Update(map[string]interface{}{"summary": "test"})
	meta := result["meta"].(map[string]interface{})
	if meta["conversation_count"] != float64(6) {
		t.Fatalf("expected count=6, got %v", meta["conversation_count"])
	}
}

func TestLTM_Delete(t *testing.T) {
	s := NewInMemoryMemoryStore()
	ltm := NewLongTermMemory(s, "test:u1", 0)
	ltm.Save(map[string]interface{}{"custom": "data", "meta": map[string]interface{}{}})
	ltm.DeleteMem()
	data, _ := ltm.Get()
	if _, ok := data["custom"]; ok {
		t.Fatal("expected default after delete")
	}
}

// ══════════════════════════════════════════════
// ConversationBuffer
// ══════════════════════════════════════════════

func TestBuffer_AddAndCount(t *testing.T) {
	s := NewInMemoryMemoryStore()
	buf := NewConversationBuffer(s, "test:u1", 3, 0)
	buf.Add("user", "hello")
	buf.Add("assistant", "hi")
	n, _ := buf.Count()
	if n != 2 {
		t.Fatalf("expected 2, got %d", n)
	}
}

func TestBuffer_ShouldExtract(t *testing.T) {
	s := NewInMemoryMemoryStore()
	buf := NewConversationBuffer(s, "test:u1", 3, 24*3600*1e9)

	should, _ := buf.ShouldExtract()
	if should {
		t.Fatal("empty buffer should not trigger")
	}

	buf.Add("user", "a")
	buf.Add("user", "b")
	should, _ = buf.ShouldExtract()
	if !should {
		t.Fatal("first time with data should trigger")
	}

	buf.GetAndClear()
	buf.Add("user", "c")
	should, _ = buf.ShouldExtract()
	if should {
		t.Fatal("only 1 message, should not trigger by count")
	}

	buf.Add("user", "d")
	buf.Add("user", "e")
	should, _ = buf.ShouldExtract()
	if !should {
		t.Fatal("3 >= trigger_count, should trigger")
	}
}

func TestBuffer_GetAndClear(t *testing.T) {
	s := NewInMemoryMemoryStore()
	buf := NewConversationBuffer(s, "test:u1", 5, 0)
	buf.Add("user", "hello")
	msgs, _ := buf.GetAndClear()
	if len(msgs) != 1 || msgs[0]["role"] != "user" {
		t.Fatalf("unexpected: %v", msgs)
	}
	n, _ := buf.Count()
	if n != 0 {
		t.Fatal("expected empty after clear")
	}
}

// ══════════════════════════════════════════════
// MemoryExtractor
// ══════════════════════════════════════════════

func TestParseJSONResponse_Clean(t *testing.T) {
	r := ParseJSONResponse(`{"basic_info": {"age": 25}}`)
	bi := r["basic_info"].(map[string]interface{})
	if bi["age"] != float64(25) {
		t.Fatal("parse failed")
	}
}

func TestParseJSONResponse_CodeBlock(t *testing.T) {
	r := ParseJSONResponse("```json\n{\"key\": \"val\"}\n```")
	if r["key"] != "val" {
		t.Fatal("parse code block failed")
	}
}

func TestParseJSONResponse_Invalid(t *testing.T) {
	r := ParseJSONResponse("not json")
	if len(r) != 0 {
		t.Fatal("expected empty for invalid")
	}
}

func TestLLMExtractor_Extract(t *testing.T) {
	ext := NewLLMMemoryExtractor(func(prompt string) (string, error) {
		return `{"basic_info": {"age": 30}, "interests": ["hiking"]}`, nil
	}, "")

	result, err := ext.Extract(
		[]map[string]string{{"role": "user", "content": "I'm 30 and love hiking"}},
		map[string]interface{}{},
	)
	if err != nil {
		t.Fatal(err)
	}
	bi := result["basic_info"].(map[string]interface{})
	if bi["age"] != float64(30) {
		t.Fatal("expected age=30")
	}
}

func TestLLMExtractor_EmptyConversations(t *testing.T) {
	ext := NewLLMMemoryExtractor(func(prompt string) (string, error) {
		return "{}", nil
	}, "")
	result, _ := ext.Extract(nil, nil)
	if len(result) != 0 {
		t.Fatal("expected empty for nil conversations")
	}
}

// ══════════════════════════════════════════════
// MemoryFormatter
// ══════════════════════════════════════════════

func TestFormatter_WithData(t *testing.T) {
	m := map[string]interface{}{
		"basic_info": map[string]interface{}{"age": float64(25), "location": "Shanghai"},
		"interests":  []interface{}{"coding"},
		"summary":    "A developer",
		"meta":       map[string]interface{}{"conversation_count": float64(10)},
	}
	result := FormatMemoryForPrompt(m, nil, "")
	if result == "" {
		t.Fatal("expected non-empty prompt")
	}
}

func TestFormatter_Empty(t *testing.T) {
	result := FormatMemoryForPrompt(nil, nil, "")
	if result != "" {
		t.Fatal("expected empty for nil memory")
	}
}

func TestFormatter_CustomTemplate(t *testing.T) {
	m := map[string]interface{}{
		"basic_info": map[string]interface{}{"age": float64(25)},
		"meta":       map[string]interface{}{},
	}
	result := FormatMemoryForPrompt(m, nil, "USER: {long_term_text}")
	if len(result) < 5 || result[:5] != "USER:" {
		t.Fatalf("expected custom template, got %s", result)
	}
}

// ══════════════════════════════════════════════
// MemorySession (integration)
// ══════════════════════════════════════════════

func TestSession_Load(t *testing.T) {
	s := NewMemorySession("agent1", "user1", NewInMemoryMemoryStore())
	ctx, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(ctx.ShortTerm) != 0 {
		t.Fatal("expected empty history")
	}
	if _, ok := ctx.LongTerm["basic_info"]; !ok {
		t.Fatal("expected default schema")
	}
}

func TestSession_AddMessage(t *testing.T) {
	s := NewMemorySession("agent1", "user1", NewInMemoryMemoryStore())
	s.AddMessage("user", "hello")
	s.AddMessage("assistant", "hi")
	ctx, _ := s.Load()
	if len(ctx.ShortTerm) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(ctx.ShortTerm))
	}
}

func TestSession_NamespaceIsolation(t *testing.T) {
	store := NewInMemoryMemoryStore()
	s1 := NewMemorySession("agent1", "user1", store)
	s2 := NewMemorySession("agent2", "user1", store)
	s1.AddMessage("user", "from agent1")
	s2.AddMessage("user", "from agent2")
	c1, _ := s1.Load()
	c2, _ := s2.Load()
	if len(c1.ShortTerm) != 1 || len(c2.ShortTerm) != 1 {
		t.Fatal("namespace isolation failed")
	}
}

func TestSession_FormatForPrompt(t *testing.T) {
	s := NewMemorySession("a1", "u1", NewInMemoryMemoryStore())
	s.Load()
	s.UpdateLongTerm(map[string]interface{}{
		"basic_info": map[string]interface{}{"age": float64(25)},
		"interests":  []interface{}{"coding"},
	})
	prompt := s.FormatForPrompt("")
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
}

func TestSession_ExtractNoExtractor(t *testing.T) {
	s := NewMemorySession("a1", "u1", NewInMemoryMemoryStore())
	result := s.ExtractIfNeeded()
	if result != nil {
		t.Fatal("expected nil without extractor")
	}
}

func TestSession_ExtractWithExtractor(t *testing.T) {
	store := NewInMemoryMemoryStore()
	s := NewMemorySessionWithOptions("a1", "u1", store, 40, 0, 2, 24*3600*1e9)
	s.SetExtractor(NewLLMMemoryExtractor(func(prompt string) (string, error) {
		return `{"basic_info": {"age": 30}}`, nil
	}, ""))

	s.AddMessage("user", "I'm 30")
	s.AddMessage("assistant", "Got it")
	result := s.ExtractIfNeeded()
	if result == nil {
		t.Fatal("expected extraction result")
	}
	bi := result["basic_info"].(map[string]interface{})
	if bi["age"] != float64(30) {
		t.Fatal("expected age=30")
	}
}

func TestSession_ClearHistory(t *testing.T) {
	s := NewMemorySession("a1", "u1", NewInMemoryMemoryStore())
	s.AddMessage("user", "x")
	s.ClearHistory()
	ctx, _ := s.Load()
	if len(ctx.ShortTerm) != 0 {
		t.Fatal("expected empty after clear")
	}
}

func TestSession_ClearAll(t *testing.T) {
	s := NewMemorySession("a1", "u1", NewInMemoryMemoryStore())
	s.AddMessage("user", "x")
	s.UpdateLongTerm(map[string]interface{}{"interests": []interface{}{"test"}})
	s.Working.Set("k", "v")
	s.ClearAll()
	ctx, _ := s.Load()
	if len(ctx.ShortTerm) != 0 {
		t.Fatal("expected empty history")
	}
	if s.Working.Len() != 0 {
		t.Fatal("expected empty working")
	}
}

func TestSession_UpdateLongTerm(t *testing.T) {
	s := NewMemorySession("a1", "u1", NewInMemoryMemoryStore())
	result, _ := s.UpdateLongTerm(map[string]interface{}{"basic_info": map[string]interface{}{"age": float64(22)}})
	bi := result["basic_info"].(map[string]interface{})
	if bi["age"] != float64(22) {
		t.Fatal("expected age=22")
	}
}

// ══════════════════════════════════════════════
// DeepMerge
// ══════════════════════════════════════════════

func TestDeepMerge_Maps(t *testing.T) {
	base := map[string]interface{}{
		"a": map[string]interface{}{"x": float64(1)},
	}
	over := map[string]interface{}{
		"a": map[string]interface{}{"y": float64(2)},
	}
	result := DeepMerge(base, over)
	a := result["a"].(map[string]interface{})
	if a["x"] != float64(1) || a["y"] != float64(2) {
		t.Fatalf("merge failed: %v", a)
	}
}

func TestDeepMerge_Lists(t *testing.T) {
	base := map[string]interface{}{"items": []interface{}{"a", "b"}}
	over := map[string]interface{}{"items": []interface{}{"b", "c"}}
	result := DeepMerge(base, over)
	items := result["items"].([]interface{})
	if len(items) != 3 {
		t.Fatalf("expected 3, got %v", items)
	}
}

func TestDeepMerge_SkipNil(t *testing.T) {
	base := map[string]interface{}{"x": "keep"}
	over := map[string]interface{}{"x": nil}
	result := DeepMerge(base, over)
	if result["x"] != "keep" {
		t.Fatal("nil should not overwrite")
	}
}

// ══════════════════════════════════════════════
// MemoryMessage
// ══════════════════════════════════════════════

func TestMemoryMessage_JSON(t *testing.T) {
	msg := NewMemoryMessage("user", "hello")
	data, _ := json.Marshal(msg)
	var parsed MemoryMessage
	json.Unmarshal(data, &parsed)
	if parsed.Role != "user" || parsed.Content != "hello" {
		t.Fatalf("unexpected: %+v", parsed)
	}
}
