package persona

import (
	"strings"
	"testing"
)

func TestCompilerCompile_BasicSuccess(t *testing.T) {
	spec := &PersonaSpec{
		Name:              "测试人格",
		Traits:            []string{"真诚", "稳定"},
		Hobbies:           []string{"瑜伽", "写作"},
		Profession:        "工程师",
		RelationshipStyle: "friend",
		Tone:              "warm",
	}

	cfg, err := NewCompiler().Compile(spec)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil runtime config")
	}
	if !strings.HasPrefix(cfg.PersonaID, "p_") {
		t.Fatalf("unexpected persona_id: %s", cfg.PersonaID)
	}
	if cfg.Version == "" || cfg.ConfigHash == "" || cfg.SpecHash == "" {
		t.Fatalf("expected version/hash fields, got %+v", cfg)
	}
	if cfg.SystemPrompt == "" {
		t.Fatal("expected non-empty system prompt")
	}
	if len(cfg.StateMachine.Slots) != 6 {
		t.Fatalf("expected 6 time slots, got %d", len(cfg.StateMachine.Slots))
	}
	if len(cfg.TodayEventPool) == 0 {
		t.Fatal("expected generated event pool")
	}
}

func TestCompilerCompile_InvalidSpec(t *testing.T) {
	_, err := NewCompiler().Compile(&PersonaSpec{})
	if err == nil {
		t.Fatal("expected error for invalid spec")
	}
	if !strings.Contains(err.Error(), "normalize") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateStateMachine_DistributionAndDefaults(t *testing.T) {
	spec := &PersonaSpec{
		Name:       "状态机测试",
		Traits:     []string{"稳定"},
		Hobbies:    []string{"瑜伽", "写作", "猫"},
		Profession: "产品经理",
	}

	sm := GenerateStateMachine(spec)
	if sm.Timezone != "Asia/Shanghai" {
		t.Fatalf("unexpected timezone: %s", sm.Timezone)
	}
	if len(sm.Slots) != 6 {
		t.Fatalf("expected 6 slots, got %d", len(sm.Slots))
	}

	morning := sm.Slots[0]
	if !contains(morning.Activities, "瑜伽") {
		t.Fatalf("expected morning activities include 瑜伽, got %v", morning.Activities)
	}

	noonWork := sm.Slots[1]
	if !contains(noonWork.Activities, "工作") {
		t.Fatalf("expected work activity in noon slot, got %v", noonWork.Activities)
	}

	afternoon := sm.Slots[3]
	if !contains(afternoon.Activities, "写作") {
		t.Fatalf("expected afternoon activities include 写作, got %v", afternoon.Activities)
	}
	if len(afternoon.Activities) != len(afternoon.Weights) {
		t.Fatalf("activities/weights length mismatch: %d/%d", len(afternoon.Activities), len(afternoon.Weights))
	}
}

func TestCalculateMood_BoundaryAndLabel(t *testing.T) {
	high := CalculateMood("happy", 200)
	if high.Value != 100 {
		t.Fatalf("expected clamped high value 100, got %.2f", high.Value)
	}

	lowEnergy := CalculateMood("tired", 10)
	if lowEnergy.Label != "有点累但心里很平静" {
		t.Fatalf("unexpected label for low energy mood: %s", lowEnergy.Label)
	}

	normal := CalculateMood("calm", 80)
	if normal.Label != "挺放松的" {
		t.Fatalf("unexpected label for calm mood: %s", normal.Label)
	}
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}
