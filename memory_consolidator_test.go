package agentsdk

import (
	"encoding/json"
	"testing"
)

func TestDefaultConsolidator_Add(t *testing.T) {
	c := NewDefaultMemoryConsolidator()
	existing := map[string]interface{}{
		"basic_info": map[string]interface{}{"age": float64(22)},
	}
	ops := []MemoryOperation{
		{Action: MemoryActionAdd, Key: "basic_info.location", Value: "Shanghai"},
	}
	result, err := c.Consolidate(ops, existing)
	if err != nil {
		t.Fatal(err)
	}
	bi := result["basic_info"].(map[string]interface{})
	if bi["location"] != "Shanghai" {
		t.Errorf("expected Shanghai, got %v", bi["location"])
	}
	if bi["age"] != float64(22) {
		t.Errorf("existing field should be preserved, got %v", bi["age"])
	}
}

func TestDefaultConsolidator_Update(t *testing.T) {
	c := NewDefaultMemoryConsolidator()
	existing := map[string]interface{}{
		"basic_info": map[string]interface{}{"age": float64(22)},
	}
	ops := []MemoryOperation{
		{Action: MemoryActionUpdate, Key: "basic_info.age", Value: float64(25)},
	}
	result, err := c.Consolidate(ops, existing)
	if err != nil {
		t.Fatal(err)
	}
	bi := result["basic_info"].(map[string]interface{})
	if bi["age"] != float64(25) {
		t.Errorf("expected 25, got %v", bi["age"])
	}
}

func TestDefaultConsolidator_Delete(t *testing.T) {
	c := NewDefaultMemoryConsolidator()
	existing := map[string]interface{}{
		"basic_info": map[string]interface{}{"age": float64(22), "location": "Beijing"},
	}
	ops := []MemoryOperation{
		{Action: MemoryActionDelete, Key: "basic_info.location"},
	}
	result, err := c.Consolidate(ops, existing)
	if err != nil {
		t.Fatal(err)
	}
	bi := result["basic_info"].(map[string]interface{})
	if _, exists := bi["location"]; exists {
		t.Error("location should have been deleted")
	}
	if bi["age"] != float64(22) {
		t.Error("age should be preserved")
	}
}

func TestDefaultConsolidator_Noop(t *testing.T) {
	c := NewDefaultMemoryConsolidator()
	existing := map[string]interface{}{"summary": "test user"}
	ops := []MemoryOperation{
		{Action: MemoryActionNoop, Key: "summary"},
	}
	result, err := c.Consolidate(ops, existing)
	if err != nil {
		t.Fatal(err)
	}
	if result["summary"] != "test user" {
		t.Error("NOOP should not change anything")
	}
}

func TestDefaultConsolidator_DeepPath(t *testing.T) {
	c := NewDefaultMemoryConsolidator()
	existing := map[string]interface{}{}
	ops := []MemoryOperation{
		{Action: MemoryActionAdd, Key: "life_context.goals", Value: []interface{}{"learn Go"}},
	}
	result, err := c.Consolidate(ops, existing)
	if err != nil {
		t.Fatal(err)
	}
	lc, ok := result["life_context"].(map[string]interface{})
	if !ok {
		t.Fatal("life_context should be created")
	}
	goals, ok := lc["goals"].([]interface{})
	if !ok || len(goals) != 1 {
		t.Errorf("unexpected goals: %v", lc["goals"])
	}
}

func TestDefaultConsolidator_MixedOps(t *testing.T) {
	c := NewDefaultMemoryConsolidator()
	existing := map[string]interface{}{
		"basic_info": map[string]interface{}{"age": float64(22), "location": "Beijing"},
		"summary":    "a developer",
	}
	ops := []MemoryOperation{
		{Action: MemoryActionUpdate, Key: "basic_info.age", Value: float64(23)},
		{Action: MemoryActionDelete, Key: "basic_info.location"},
		{Action: MemoryActionAdd, Key: "basic_info.occupation", Value: "engineer"},
		{Action: MemoryActionNoop, Key: "summary"},
	}
	result, err := c.Consolidate(ops, existing)
	if err != nil {
		t.Fatal(err)
	}
	bi := result["basic_info"].(map[string]interface{})
	if bi["age"] != float64(23) {
		t.Error("age should be updated")
	}
	if _, exists := bi["location"]; exists {
		t.Error("location should be deleted")
	}
	if bi["occupation"] != "engineer" {
		t.Error("occupation should be added")
	}
	if result["summary"] != "a developer" {
		t.Error("summary should be unchanged")
	}
}

func TestParseOperationsResponse_Clean(t *testing.T) {
	input := `[{"action":"ADD","key":"basic_info.age","value":25,"reason":"user said 25"}]`
	ops, err := ParseOperationsResponse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 || ops[0].Action != MemoryActionAdd {
		t.Errorf("unexpected ops: %+v", ops)
	}
}

func TestParseOperationsResponse_CodeBlock(t *testing.T) {
	input := "```json\n" + `[{"action":"UPDATE","key":"summary","value":"likes coding"}]` + "\n```"
	ops, err := ParseOperationsResponse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 || ops[0].Action != MemoryActionUpdate {
		t.Errorf("unexpected ops: %+v", ops)
	}
}

func TestParseOperationsResponse_SurroundingText(t *testing.T) {
	input := "Here are the results:\n" +
		`[{"action":"DELETE","key":"life_context.concerns","reason":"resolved"}]` +
		"\nDone."
	ops, err := ParseOperationsResponse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 || ops[0].Action != MemoryActionDelete {
		t.Errorf("unexpected ops: %+v", ops)
	}
}

func TestParseOperationsResponse_Invalid(t *testing.T) {
	_, err := ParseOperationsResponse("not json")
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestParseOperationsResponse_FiltersInvalid(t *testing.T) {
	input := `[
		{"action":"ADD","key":"a","value":1},
		{"action":"INVALID","key":"b","value":2},
		{"action":"add","key":"","value":3}
	]`
	ops, err := ParseOperationsResponse(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 {
		t.Errorf("expected 1 valid op, got %d", len(ops))
	}
}

func TestConsolidatingExtractor_Extract(t *testing.T) {
	mockLLM := func(prompt string) (string, error) {
		return `[{"action":"ADD","key":"basic_info.age","value":25}]`, nil
	}
	ext := NewConsolidatingExtractor(mockLLM, nil)
	result, err := ext.Extract(
		[]map[string]string{{"role": "user", "content": "I'm 25"}},
		map[string]interface{}{"basic_info": map[string]interface{}{}},
	)
	if err != nil {
		t.Fatal(err)
	}
	bi := result["basic_info"].(map[string]interface{})
	if bi["age"] != float64(25) {
		t.Errorf("expected 25, got %v", bi["age"])
	}
}

func TestConsolidatingExtractor_FallbackToDeepMerge(t *testing.T) {
	mockLLM := func(prompt string) (string, error) {
		return `{"basic_info": {"age": 30}}`, nil
	}
	ext := NewConsolidatingExtractor(mockLLM, nil)
	result, err := ext.Extract(
		[]map[string]string{{"role": "user", "content": "I'm 30"}},
		map[string]interface{}{},
	)
	if err != nil {
		t.Fatal(err)
	}
	bi := result["basic_info"].(map[string]interface{})
	if bi["age"] != float64(30) {
		t.Errorf("expected 30, got %v", bi["age"])
	}
}

func TestConsolidatingExtractor_Empty(t *testing.T) {
	ext := NewConsolidatingExtractor(func(string) (string, error) {
		t.Fatal("should not be called")
		return "", nil
	}, nil)
	result, err := ext.Extract(nil, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 0 {
		t.Error("expected empty result")
	}
}

func TestSetNestedValue_Deep(t *testing.T) {
	m := map[string]interface{}{}
	setNestedValue(m, "a.b.c", "deep")
	a := m["a"].(map[string]interface{})
	b := a["b"].(map[string]interface{})
	if b["c"] != "deep" {
		t.Error("deep nested set failed")
	}
}

func TestDeleteNestedValue_Missing(t *testing.T) {
	m := map[string]interface{}{"a": "val"}
	deleteNestedValue(m, "x.y.z")
	if m["a"] != "val" {
		t.Error("should not affect existing data")
	}
}

func TestMemoryOperation_JSON(t *testing.T) {
	op := MemoryOperation{Action: MemoryActionAdd, Key: "basic_info.age", Value: 25, Reason: "test"}
	data, _ := json.Marshal(op)
	var parsed MemoryOperation
	json.Unmarshal(data, &parsed)
	if parsed.Action != MemoryActionAdd || parsed.Key != "basic_info.age" {
		t.Errorf("unexpected: %+v", parsed)
	}
}
