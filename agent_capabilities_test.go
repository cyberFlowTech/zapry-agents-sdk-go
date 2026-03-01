package agentsdk

import (
	"testing"
)

func TestAllTags_Dedup(t *testing.T) {
	caps := &AgentCapabilities{
		Skills: []SkillSpec{
			{Name: "塔罗占卜", Tags: []string{"塔罗", "占卜"}},
			{Name: "心理陪伴", Tags: []string{"占卜", "陪伴"}}, // "占卜" 重复
		},
	}
	tags := caps.AllTags()
	if len(tags) != 3 {
		t.Errorf("expected 3 unique tags, got %d: %v", len(tags), tags)
	}
}

func TestAllTags_Nil(t *testing.T) {
	var caps *AgentCapabilities
	tags := caps.AllTags()
	if tags != nil {
		t.Errorf("expected nil, got %v", tags)
	}
}

func TestHasTool(t *testing.T) {
	caps := &AgentCapabilities{
		ToolManifest: []ToolSpec{
			{Name: "draw_card"},
			{Name: "get_history"},
		},
	}
	if !caps.HasTool("draw_card") {
		t.Error("expected HasTool(draw_card) = true")
	}
	if caps.HasTool("nonexistent") {
		t.Error("expected HasTool(nonexistent) = false")
	}
}

func TestHasTool_Nil(t *testing.T) {
	var caps *AgentCapabilities
	if caps.HasTool("any") {
		t.Error("nil caps should return false")
	}
}

func TestFindSkillByTool_ViaGrants(t *testing.T) {
	caps := &AgentCapabilities{
		Skills: []SkillSpec{
			{
				Name:       "塔罗占卜",
				ToolGrants: []ToolGrant{{ToolName: "draw_card"}},
			},
			{
				Name:  "心理陪伴",
				Tools: []string{"get_memory"},
			},
		},
	}
	skill := caps.FindSkillByTool("draw_card")
	if skill == nil || skill.Name != "塔罗占卜" {
		t.Errorf("expected 塔罗占卜, got %v", skill)
	}
}

func TestFindSkillByTool_ViaToolsList(t *testing.T) {
	caps := &AgentCapabilities{
		Skills: []SkillSpec{
			{Name: "心理陪伴", Tools: []string{"get_memory"}},
		},
	}
	skill := caps.FindSkillByTool("get_memory")
	if skill == nil || skill.Name != "心理陪伴" {
		t.Errorf("expected 心理陪伴, got %v", skill)
	}
}

func TestFindSkillByTool_NotFound(t *testing.T) {
	caps := &AgentCapabilities{
		Skills: []SkillSpec{{Name: "A", Tools: []string{"x"}}},
	}
	if caps.FindSkillByTool("nonexistent") != nil {
		t.Error("expected nil")
	}
}

func TestFindToolGrant(t *testing.T) {
	caps := &AgentCapabilities{
		Skills: []SkillSpec{
			{
				Name: "塔罗占卜",
				ToolGrants: []ToolGrant{
					{ToolName: "draw_card", Tier: "free", RateLimit: &RateLimitSpec{MaxPerDay: 10}},
				},
			},
		},
	}
	grant, skillName := caps.FindToolGrant("draw_card")
	if grant == nil {
		t.Fatal("expected grant, got nil")
	}
	if skillName != "塔罗占卜" {
		t.Errorf("expected skill 塔罗占卜, got %s", skillName)
	}
	if grant.Tier != "free" {
		t.Errorf("expected tier free, got %s", grant.Tier)
	}
	if grant.RateLimit.MaxPerDay != 10 {
		t.Errorf("expected MaxPerDay 10, got %d", grant.RateLimit.MaxPerDay)
	}
}

func TestCheckToolGrant_NilCaps(t *testing.T) {
	d := CheckToolGrant(nil, "any_tool")
	if !d.Allowed {
		t.Error("nil caps should allow all")
	}
}

func TestCheckToolGrant_EmptyManifest(t *testing.T) {
	d := CheckToolGrant(&AgentCapabilities{}, "any_tool")
	if !d.Allowed {
		t.Error("empty manifest should allow all")
	}
}

func TestCheckToolGrant_Denied(t *testing.T) {
	caps := &AgentCapabilities{
		ToolManifest: []ToolSpec{{Name: "draw_card"}},
	}
	d := CheckToolGrant(caps, "nonexistent")
	if d.Allowed {
		t.Error("expected denied for tool not in manifest")
	}
	if d.DenyReason == "" {
		t.Error("expected DenyReason")
	}
}

func TestCheckToolGrant_AllowedWithGrant(t *testing.T) {
	caps := &AgentCapabilities{
		Skills: []SkillSpec{
			{
				Name:       "塔罗占卜",
				ToolGrants: []ToolGrant{{ToolName: "draw_card", Tier: "premium"}},
			},
		},
		ToolManifest: []ToolSpec{{Name: "draw_card"}},
	}
	d := CheckToolGrant(caps, "draw_card")
	if !d.Allowed {
		t.Error("expected allowed")
	}
	if d.Grant == nil {
		t.Error("expected grant to be set")
	}
	if d.SkillName != "塔罗占卜" {
		t.Errorf("expected skill 塔罗占卜, got %s", d.SkillName)
	}
	if d.Grant.Tier != "premium" {
		t.Errorf("expected tier premium, got %s", d.Grant.Tier)
	}
}

func TestCheckToolGrant_AllowedWithoutGrant(t *testing.T) {
	caps := &AgentCapabilities{
		Skills:       []SkillSpec{{Name: "心理陪伴", Tools: []string{"get_memory"}}},
		ToolManifest: []ToolSpec{{Name: "get_memory"}},
	}
	d := CheckToolGrant(caps, "get_memory")
	if !d.Allowed {
		t.Error("expected allowed")
	}
	if d.SkillName != "心理陪伴" {
		t.Errorf("expected skill 心理陪伴, got %s", d.SkillName)
	}
	if d.Grant != nil {
		t.Error("expected no grant (tool in Tools list, not ToolGrants)")
	}
}

func TestToRoutingView(t *testing.T) {
	card := AgentCardPublic{
		AgentID:     "elena",
		DisplayName: "林晚晴",
		Description: "塔罗解读师",
		Capabilities: &AgentCapabilities{
			Skills: []SkillSpec{
				{
					Name:        "塔罗占卜",
					Description: "通过塔罗牌解读...",
					Tags:        []string{"塔罗", "占卜"},
					Tools:       []string{"draw_card"},
					Tier:        "free",
				},
			},
			ToolManifest: []ToolSpec{
				{Name: "draw_card", Category: "divination"},
			},
		},
	}
	view := ToRoutingView(card)
	if view.AgentID != "elena" {
		t.Errorf("expected elena, got %s", view.AgentID)
	}
	if len(view.SkillSummaries) != 1 {
		t.Fatalf("expected 1 skill summary, got %d", len(view.SkillSummaries))
	}
	ss := view.SkillSummaries[0]
	if ss.Name != "塔罗占卜" {
		t.Errorf("expected 塔罗占卜, got %s", ss.Name)
	}
	if len(ss.ToolCategories) != 1 || ss.ToolCategories[0] != "divination" {
		t.Errorf("expected [divination], got %v", ss.ToolCategories)
	}
}

func TestAllToolNames(t *testing.T) {
	caps := &AgentCapabilities{
		ToolManifest: []ToolSpec{{Name: "a"}, {Name: "b"}, {Name: "c"}},
	}
	names := caps.AllToolNames()
	if len(names) != 3 {
		t.Errorf("expected 3, got %d", len(names))
	}
}
