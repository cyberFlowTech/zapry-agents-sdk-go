package agentsdk

import (
	"testing"
)

func dummyHandler(ctx *ToolContext, args map[string]interface{}) (interface{}, error) {
	return "ok", nil
}

func TestBuilder_Basic(t *testing.T) {
	cfg, err := NewAgentBuilder("elena", "林晚晴").
		Description("塔罗解读师").
		Skill("塔罗占卜", "通过塔罗牌解读",
			SkillTools("draw_card"),
			SkillTags("塔罗", "占卜"),
			SkillTier("free"),
		).
		Tool("draw_card", "抽取塔罗牌", dummyHandler,
			ToolParam{Name: "spread", Type: "string", Required: true},
		).
		SystemPrompt("你是林晚晴").
		Build()

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if cfg.Card.AgentID != "elena" {
		t.Errorf("expected agent_id elena, got %s", cfg.Card.AgentID)
	}
	if cfg.Card.DisplayName != "林晚晴" {
		t.Errorf("expected display_name 林晚晴, got %s", cfg.Card.DisplayName)
	}
	if cfg.Card.Capabilities == nil {
		t.Fatal("expected Capabilities to be set")
	}
	if len(cfg.Card.Capabilities.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(cfg.Card.Capabilities.Skills))
	}
	if cfg.Card.Capabilities.Skills[0].Tier != "free" {
		t.Errorf("expected tier free, got %s", cfg.Card.Capabilities.Skills[0].Tier)
	}

	// 向后兼容：Skills 自动填充
	tags := cfg.Card.Skills
	if len(tags) != 2 {
		t.Errorf("expected 2 auto-filled Skills tags, got %d: %v", len(tags), tags)
	}

	// ToolRegistry 可执行
	if cfg.ToolReg.Len() != 1 {
		t.Errorf("expected 1 tool in registry, got %d", cfg.ToolReg.Len())
	}

	// ToolManifest 参数摘要
	if len(cfg.Card.Capabilities.ToolManifest) != 1 {
		t.Fatal("expected 1 tool in manifest")
	}
	ts := cfg.Card.Capabilities.ToolManifest[0]
	if len(ts.ParamsSummary) != 1 || ts.ParamsSummary[0] != "spread:string(required)" {
		t.Errorf("expected params_summary [spread:string(required)], got %v", ts.ParamsSummary)
	}
}

func TestBuilder_WithCategory(t *testing.T) {
	cfg, err := NewAgentBuilder("test", "Test").
		Skill("S1", "desc", SkillTools("t1")).
		ToolWithCategory("t1", "tool1", "divination", dummyHandler).
		Build()

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if cfg.Card.Capabilities.ToolManifest[0].Category != "divination" {
		t.Errorf("expected category divination, got %s", cfg.Card.Capabilities.ToolManifest[0].Category)
	}
}

func TestBuilder_WithKnowledge(t *testing.T) {
	cfg, err := NewAgentBuilder("test", "Test").
		Skill("S1", "desc",
			SkillTools("t1"),
			SkillKnowledge("k1"),
		).
		Tool("t1", "tool1", dummyHandler).
		Knowledge("k1", "知识库", "static", "测试知识").
		Build()

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if len(cfg.Card.Capabilities.Knowledge) != 1 {
		t.Fatalf("expected 1 knowledge, got %d", len(cfg.Card.Capabilities.Knowledge))
	}
	if cfg.Card.Capabilities.Knowledge[0].ID != "k1" {
		t.Errorf("expected k1, got %s", cfg.Card.Capabilities.Knowledge[0].ID)
	}
}

func TestBuilder_WithToolGrants(t *testing.T) {
	cfg, err := NewAgentBuilder("test", "Test").
		Skill("S1", "desc",
			SkillToolGrants(ToolGrant{
				ToolName:  "t1",
				Tier:      "premium",
				RateLimit: &RateLimitSpec{MaxPerDay: 5},
			}),
		).
		Tool("t1", "tool1", dummyHandler).
		Build()

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	grants := cfg.Card.Capabilities.Skills[0].ToolGrants
	if len(grants) != 1 {
		t.Fatalf("expected 1 grant, got %d", len(grants))
	}
	if grants[0].Tier != "premium" {
		t.Errorf("expected premium, got %s", grants[0].Tier)
	}
}

func TestBuilder_ValidationError_MissingTool(t *testing.T) {
	_, err := NewAgentBuilder("test", "Test").
		Skill("S1", "desc", SkillTools("nonexistent")).
		Build()

	if err == nil {
		t.Fatal("expected validation error for missing tool")
	}
	t.Logf("got expected error: %v", err)
}

func TestBuilder_ValidationError_MissingToolGrant(t *testing.T) {
	_, err := NewAgentBuilder("test", "Test").
		Skill("S1", "desc",
			SkillToolGrants(ToolGrant{ToolName: "nonexistent"}),
		).
		Build()

	if err == nil {
		t.Fatal("expected validation error for missing tool_grant tool")
	}
	t.Logf("got expected error: %v", err)
}

func TestBuilder_ValidationError_MissingKnowledge(t *testing.T) {
	_, err := NewAgentBuilder("test", "Test").
		Skill("S1", "desc",
			SkillTools("t1"),
			SkillKnowledge("nonexistent"),
		).
		Tool("t1", "tool1", dummyHandler).
		Build()

	if err == nil {
		t.Fatal("expected validation error for missing knowledge")
	}
	t.Logf("got expected error: %v", err)
}

func TestBuilder_ValidationError_DuplicateSkill(t *testing.T) {
	_, err := NewAgentBuilder("test", "Test").
		Skill("S1", "desc").
		Skill("S1", "desc2").
		Build()

	if err == nil {
		t.Fatal("expected validation error for duplicate skill")
	}
	t.Logf("got expected error: %v", err)
}

func TestBuilder_ValidationError_EmptyID(t *testing.T) {
	_, err := NewAgentBuilder("", "Test").Build()
	if err == nil {
		t.Fatal("expected validation error for empty id")
	}
}

func TestBuilder_ValidationError_EmptyDisplayName(t *testing.T) {
	_, err := NewAgentBuilder("test", "").Build()
	if err == nil {
		t.Fatal("expected validation error for empty display_name")
	}
}

func TestBuilder_EffectiveSkillTags(t *testing.T) {
	cfg, _ := NewAgentBuilder("test", "Test").
		Skill("S1", "d", SkillTags("tag1", "tag2")).
		Skill("S2", "d", SkillTags("tag2", "tag3")).
		Build()

	tags := cfg.Card.EffectiveSkillTags()
	if len(tags) != 3 {
		t.Errorf("expected 3 effective tags, got %d: %v", len(tags), tags)
	}
}

func TestBuilder_BackwardCompat_NoCapabilities(t *testing.T) {
	card := AgentCardPublic{
		Skills: []string{"old_tag1", "old_tag2"},
	}
	tags := card.EffectiveSkillTags()
	if len(tags) != 2 {
		t.Errorf("expected 2 old skills, got %d", len(tags))
	}
}

func TestBuilder_MultipleTools(t *testing.T) {
	cfg, err := NewAgentBuilder("zhouqi", "周奇").
		Description("命理师").
		Skill("周易六爻", "铜钱法起卦",
			SkillTools("divine_yijing"),
			SkillTags("周易", "六爻"),
		).
		Skill("八字排盘", "四柱分析",
			SkillTools("calculate_bazi"),
			SkillTags("八字", "命理"),
		).
		Tool("divine_yijing", "周易占卜", dummyHandler).
		Tool("calculate_bazi", "八字排盘", dummyHandler).
		Build()

	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}
	if len(cfg.Card.Capabilities.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(cfg.Card.Capabilities.Skills))
	}
	if cfg.ToolReg.Len() != 2 {
		t.Errorf("expected 2 tools, got %d", cfg.ToolReg.Len())
	}
	tags := cfg.Card.EffectiveSkillTags()
	if len(tags) != 4 {
		t.Errorf("expected 4 tags, got %d: %v", len(tags), tags)
	}
}
