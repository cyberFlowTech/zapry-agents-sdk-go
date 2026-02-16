package agentsdk

import (
	"context"
	"testing"
	"time"
)

func makeTestCard(id, ownerID string, opts ...func(*AgentCardPublic)) AgentCardPublic {
	c := AgentCardPublic{AgentID: id, Name: id, OwnerID: ownerID, Visibility: "public", SafetyLevel: "medium", HandoffPolicyStr: "auto"}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}

func makeTestRT(id, ownerID, response string, opts ...func(*AgentCardPublic)) *AgentRuntimeConfig {
	card := makeTestCard(id, ownerID, opts...)
	return &AgentRuntimeConfig{
		Card:         card,
		LLMFn:        func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) { return &LLMMessage{Content: response}, nil },
		SystemPrompt: "test",
		MaxTurns:     5,
	}
}

// ══════════════════════════════════════════════
// Policy
// ══════════════════════════════════════════════

func TestPolicy_CheckAccessDeny(t *testing.T) {
	p := NewHandoffPolicy()
	card := makeTestCard("a", "dev1", func(c *AgentCardPublic) { c.HandoffPolicyStr = "deny" })
	err := p.CheckAccess(&HandoffRequestData{ToAgent: "a"}, &card)
	if err == nil || err.Code != "NOT_ALLOWED" {
		t.Fatal("expected NOT_ALLOWED")
	}
}

func TestPolicy_SafetyBlock(t *testing.T) {
	p := NewHandoffPolicy()
	card := makeTestCard("a", "dev1", func(c *AgentCardPublic) { c.SafetyLevel = "high" })
	err := p.CheckAccess(&HandoffRequestData{ToAgent: "a", RequestedMode: "tool_based"}, &card)
	if err == nil || err.Code != "SAFETY_BLOCK" {
		t.Fatal("expected SAFETY_BLOCK")
	}
}

func TestPolicy_PrivateSameOwner(t *testing.T) {
	p := NewHandoffPolicy()
	card := makeTestCard("a", "dev1", func(c *AgentCardPublic) { c.Visibility = "private" })
	err := p.CheckAccess(&HandoffRequestData{CallerOwnerID: "dev1"}, &card)
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestPolicy_PrivateDiffOwner(t *testing.T) {
	p := NewHandoffPolicy()
	card := makeTestCard("a", "dev1", func(c *AgentCardPublic) { c.Visibility = "private" })
	err := p.CheckAccess(&HandoffRequestData{CallerOwnerID: "dev2"}, &card)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPolicy_CrossOwnerBlocked(t *testing.T) {
	p := &HandoffPolicyConfig{MaxHopCount: 3, AllowCrossOwner: false}
	card := makeTestCard("a", "dev2")
	err := p.CheckAccess(&HandoffRequestData{CallerOwnerID: "dev1"}, &card)
	if err == nil {
		t.Fatal("expected cross-owner block")
	}
}

func TestPolicy_CrossOwnerAllowed(t *testing.T) {
	p := &HandoffPolicyConfig{MaxHopCount: 3, AllowCrossOwner: true}
	card := makeTestCard("a", "dev2")
	err := p.CheckAccess(&HandoffRequestData{CallerOwnerID: "dev1"}, &card)
	if err != nil {
		t.Fatal("expected nil")
	}
}

func TestPolicy_LoopOk(t *testing.T) {
	p := NewHandoffPolicy()
	err := p.CheckLoop(&HandoffRequestData{ToAgent: "b", HopCount: 1, VisitedAgents: []string{"a"}})
	if err != nil {
		t.Fatal("expected nil")
	}
}

func TestPolicy_LoopMaxHops(t *testing.T) {
	p := &HandoffPolicyConfig{MaxHopCount: 2}
	err := p.CheckLoop(&HandoffRequestData{ToAgent: "c", HopCount: 2, VisitedAgents: []string{"a", "b"}})
	if err == nil || err.Code != "LOOP_DETECTED" {
		t.Fatal("expected LOOP_DETECTED")
	}
}

func TestPolicy_LoopRevisit(t *testing.T) {
	p := NewHandoffPolicy()
	err := p.CheckLoop(&HandoffRequestData{ToAgent: "a", HopCount: 1, VisitedAgents: []string{"a", "b"}})
	if err == nil || err.Code != "LOOP_DETECTED" {
		t.Fatal("expected LOOP_DETECTED")
	}
}

// ══════════════════════════════════════════════
// Registry
// ══════════════════════════════════════════════

func TestRegistry2_RegisterGet(t *testing.T) {
	reg := NewAgentRegistry2()
	rt := makeTestRT("a", "dev1", "hello")
	reg.Register(rt)
	if reg.Get("a") != rt {
		t.Fatal("expected rt")
	}
	if reg.Get("missing") != nil {
		t.Fatal("expected nil")
	}
}

func TestRegistry2_FindBySkillVisibility(t *testing.T) {
	reg := NewAgentRegistry2()
	reg.Register(makeTestRT("pub", "dev1", "x", func(c *AgentCardPublic) { c.Skills = []string{"tarot"} }))
	reg.Register(makeTestRT("priv", "dev2", "x", func(c *AgentCardPublic) { c.Skills = []string{"tarot"}; c.Visibility = "private" }))
	found := reg.FindBySkill("tarot", "dev1", "")
	if len(found) != 1 || found[0].Card.AgentID != "pub" {
		t.Fatalf("expected only pub, got %d", len(found))
	}
}

// ══════════════════════════════════════════════
// Engine
// ══════════════════════════════════════════════

func TestEngine_BasicHandoff(t *testing.T) {
	reg := NewAgentRegistry2()
	reg.Register(makeTestRT("target", "dev1", "I'm the target"))
	engine := NewHandoffEngine(reg, nil)

	req := NewHandoffRequest("caller", "target", "test")
	req.CallerOwnerID = "dev1"
	result := engine.Handoff(context.Background(), req)
	if result.Status != "success" {
		t.Fatalf("expected success, got %s (%v)", result.Status, result.Error)
	}
}

func TestEngine_NotFound(t *testing.T) {
	reg := NewAgentRegistry2()
	engine := NewHandoffEngine(reg, nil)
	req := NewHandoffRequest("caller", "missing", "test")
	result := engine.Handoff(context.Background(), req)
	if result.Error == nil || result.Error.Code != "NOT_FOUND" {
		t.Fatal("expected NOT_FOUND")
	}
}

func TestEngine_PermissionDenied(t *testing.T) {
	reg := NewAgentRegistry2()
	reg.Register(makeTestRT("priv", "dev2", "x", func(c *AgentCardPublic) { c.Visibility = "private" }))
	engine := NewHandoffEngine(reg, nil)
	req := NewHandoffRequest("caller", "priv", "test")
	req.CallerOwnerID = "dev1"
	result := engine.Handoff(context.Background(), req)
	if result.Error == nil || result.Error.Code != "NOT_ALLOWED" {
		t.Fatal("expected NOT_ALLOWED")
	}
}

func TestEngine_LoopDetected(t *testing.T) {
	reg := NewAgentRegistry2()
	reg.Register(makeTestRT("a", "dev1", "x"))
	engine := NewHandoffEngine(reg, &HandoffPolicyConfig{MaxHopCount: 2, AllowCrossOwner: true})
	req := NewHandoffRequest("caller", "a", "test")
	req.HopCount = 2
	req.VisitedAgents = []string{"x", "y"}
	req.CallerOwnerID = "dev1"
	result := engine.Handoff(context.Background(), req)
	if result.Error == nil || result.Error.Code != "LOOP_DETECTED" {
		t.Fatal("expected LOOP_DETECTED")
	}
}

func TestEngine_Timeout(t *testing.T) {
	reg := NewAgentRegistry2()
	slowLLM := func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
		time.Sleep(5 * time.Second)
		return &LLMMessage{Content: "slow"}, nil
	}
	card := makeTestCard("slow", "dev1")
	reg.Register(&AgentRuntimeConfig{Card: card, LLMFn: slowLLM, SystemPrompt: "t", MaxTurns: 5})

	engine := NewHandoffEngine(reg, &HandoffPolicyConfig{MaxHopCount: 3, DefaultTimeout: 30000, AllowCrossOwner: true})
	req := NewHandoffRequest("caller", "slow", "test")
	req.DeadlineMs = 100
	req.CallerOwnerID = "dev1"
	result := engine.Handoff(context.Background(), req)
	if result.Error == nil || result.Error.Code != "TIMEOUT" {
		t.Fatalf("expected TIMEOUT, got %v", result)
	}
}

// ══════════════════════════════════════════════
// ReturnContract
// ══════════════════════════════════════════════

func TestReturnContract(t *testing.T) {
	r := &HandoffResultData{Output: "Hello", AgentID: "tarot", Status: "success", RequestID: "req1"}
	msg := r.ToReturnMessage("tc1")
	if msg["role"] != "tool" || msg["name"] != "handoff_result" || msg["tool_call_id"] != "tc1" {
		t.Fatalf("unexpected: %v", msg)
	}
}

// ══════════════════════════════════════════════
// IdempotencyCache
// ══════════════════════════════════════════════

func TestIdempotencyCache_Hit(t *testing.T) {
	cache := NewHandoffIdempotencyCache(time.Minute)
	r1 := &HandoffResultData{Output: "first", RequestID: "r1"}
	cache.GetOrSet("r1", r1)
	r2, hit := cache.GetOrSet("r1", &HandoffResultData{Output: "second"})
	if !hit {
		t.Fatal("expected cache hit")
	}
	if !r2.CacheHit {
		t.Fatal("expected cache_hit=true")
	}
	if r2.Output != "first" {
		t.Fatal("expected first result")
	}
}

func TestIdempotencyCache_NoID(t *testing.T) {
	cache := NewHandoffIdempotencyCache(time.Minute)
	r, hit := cache.GetOrSet("", &HandoffResultData{Output: "x"})
	if hit {
		t.Fatal("expected no cache hit without ID")
	}
	if r.Output != "x" {
		t.Fatal("expected original result")
	}
}
