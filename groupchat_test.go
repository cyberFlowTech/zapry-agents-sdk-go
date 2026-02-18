package agentsdk

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// ══════════════════════════════════════════════
// Test helpers
// ══════════════════════════════════════════════

func setupGroupChatRegistry() *AgentRegistryStore {
	registry := NewAgentRegistry2()

	registry.Register(&AgentRuntimeConfig{
		Card: AgentCardPublic{
			AgentID:       "linwanqing",
			Name:          "林晚晴",
			DisplayName:   "林晚晴",
			Description:   "温柔理性的朋友型角色",
			Skills:        []string{"聊天", "情感", "陪伴"},
			Talkativeness: 0.7,
		},
		LLMFn: func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
			return &LLMMessage{Content: "林晚晴的回复"}, nil
		},
		ToolReg:      NewToolRegistry(),
		SystemPrompt: "你是林晚晴",
		MaxTurns:     5,
	})

	registry.Register(&AgentRuntimeConfig{
		Card: AgentCardPublic{
			AgentID:       "fortuneteller",
			Name:          "运势大师",
			DisplayName:   "运势大师",
			Description:   "幽默的占卜师",
			Skills:        []string{"占卜", "塔罗", "星座", "运势"},
			Talkativeness: 0.3,
		},
		LLMFn: func(msgs []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error) {
			return &LLMMessage{Content: "运势大师的回复"}, nil
		},
		ToolReg:      NewToolRegistry(),
		SystemPrompt: "你是运势大师",
		MaxTurns:     5,
	})

	return registry
}

func setupGroupChat() *GroupChat {
	registry := setupGroupChatRegistry()
	gc := NewGroupChat(registry)

	store1 := NewInMemoryMemoryStore()
	session1 := NewMemorySession("test", "linwanqing", store1)
	gc.AddAgent("linwanqing", session1, nil)

	store2 := NewInMemoryMemoryStore()
	session2 := NewMemorySession("test", "fortuneteller", store2)
	gc.AddAgent("fortuneteller", session2, nil)

	return gc
}

// ══════════════════════════════════════════════
// Router tests
// ══════════════════════════════════════════════

func TestRoute_MentionAgent(t *testing.T) {
	registry := setupGroupChatRegistry()
	router := NewGroupChatRouter(registry)

	msg := GroupMessage{
		SenderID:        "user1",
		SenderName:      "张三",
		Content:         "帮我看看今天运势",
		MentionedAgents: []string{"fortuneteller"},
	}

	result := router.Route(msg, []string{"linwanqing", "fortuneteller"}, "", time.Time{})
	if result == nil {
		t.Fatal("expected route result for @mention")
	}
	if result.AgentID != "fortuneteller" {
		t.Fatalf("expected fortuneteller, got %s", result.AgentID)
	}
	if result.Reason != "mention" {
		t.Fatalf("expected reason=mention, got %s", result.Reason)
	}
}

func TestRoute_SkillMatch(t *testing.T) {
	registry := setupGroupChatRegistry()
	router := NewGroupChatRouter(registry)

	msg := GroupMessage{
		SenderID:   "user1",
		SenderName: "张三",
		Content:    "帮我算一卦塔罗牌",
	}

	result := router.Route(msg, []string{"linwanqing", "fortuneteller"}, "", time.Time{})
	if result == nil {
		t.Fatal("expected route result for skill match")
	}
	if result.AgentID != "fortuneteller" {
		t.Fatalf("expected fortuneteller for 塔罗, got %s", result.AgentID)
	}
	if result.Reason != "skill" {
		t.Fatalf("expected reason=skill, got %s", result.Reason)
	}
}

func TestRoute_FollowUp(t *testing.T) {
	registry := setupGroupChatRegistry()
	router := NewGroupChatRouter(registry)

	msg := GroupMessage{
		SenderID:   "user1",
		SenderName: "张三",
		Content:    "这张牌什么意思？",
	}

	// fortuneteller replied 20s ago → followup
	result := router.Route(msg, []string{"linwanqing", "fortuneteller"}, "fortuneteller", time.Now().Add(-20*time.Second))
	if result == nil {
		t.Fatal("expected followup route")
	}
	if result.AgentID != "fortuneteller" {
		t.Fatalf("expected fortuneteller for followup, got %s", result.AgentID)
	}
	if result.Reason != "followup" {
		t.Fatalf("expected reason=followup, got %s", result.Reason)
	}
}

func TestRoute_Talkativeness_High(t *testing.T) {
	registry := NewAgentRegistry2()
	registry.Register(&AgentRuntimeConfig{
		Card: AgentCardPublic{
			AgentID:       "chatty",
			Talkativeness: 0.99, // almost always responds
		},
	})
	router := NewGroupChatRouter(registry)

	// With 0.99 talkativeness, should respond most of the time
	responded := 0
	for i := 0; i < 20; i++ {
		msg := GroupMessage{SenderID: "u1", Content: fmt.Sprintf("msg %d", i)}
		result := router.Route(msg, []string{"chatty"}, "", time.Time{})
		if result != nil {
			responded++
		}
	}
	if responded < 10 {
		t.Fatalf("expected high talkativeness agent to respond often, got %d/20", responded)
	}
}

func TestRoute_Talkativeness_Zero(t *testing.T) {
	registry := NewAgentRegistry2()
	registry.Register(&AgentRuntimeConfig{
		Card: AgentCardPublic{
			AgentID:       "silent",
			Talkativeness: 0.0,
		},
	})
	router := NewGroupChatRouter(registry)

	// With 0.0 talkativeness, should never respond (unless @mentioned)
	for i := 0; i < 20; i++ {
		msg := GroupMessage{SenderID: "u1", Content: fmt.Sprintf("msg %d", i)}
		result := router.Route(msg, []string{"silent"}, "", time.Time{})
		if result != nil && result.Reason == "talkativeness" {
			t.Fatal("zero talkativeness should never trigger")
		}
	}
}

func TestRoute_NoMatch_Silent(t *testing.T) {
	registry := NewAgentRegistry2()
	registry.Register(&AgentRuntimeConfig{
		Card: AgentCardPublic{
			AgentID:       "agent1",
			Skills:        []string{"编程"},
			Talkativeness: 0.01, // very low
		},
	})
	router := NewGroupChatRouter(registry)

	// Unrelated message + very low talkativeness → mostly silent
	silentCount := 0
	for i := 0; i < 50; i++ {
		msg := GroupMessage{SenderID: "u1", Content: "今天天气不错"}
		result := router.Route(msg, []string{"agent1"}, "", time.Time{})
		if result == nil {
			silentCount++
		}
	}
	if silentCount < 30 {
		t.Fatalf("expected mostly silent with low talkativeness, got silent %d/50", silentCount)
	}
}

// ══════════════════════════════════════════════
// Policy tests
// ══════════════════════════════════════════════

func TestPolicy_Cooldown(t *testing.T) {
	p := DefaultSpeakingPolicy()
	now := time.Now()

	p.RecordSpeak("agent1", now)

	// 5s later: still in cooldown
	if p.CanSpeak("agent1", false, now.Add(5*time.Second)) {
		t.Fatal("should be in cooldown")
	}
}

func TestPolicy_CooldownExpired(t *testing.T) {
	p := DefaultSpeakingPolicy()
	now := time.Now()

	p.SetCurrentMessage("msg1")
	p.RecordSpeak("agent1", now)

	// 31s later, new message: cooldown expired
	p.SetCurrentMessage("msg2")
	if !p.CanSpeak("agent1", false, now.Add(31*time.Second)) {
		t.Fatal("cooldown should be expired")
	}
}

func TestPolicy_MentionBypassesCooldown(t *testing.T) {
	p := DefaultSpeakingPolicy()
	now := time.Now()

	p.RecordSpeak("agent1", now)

	// In cooldown but @mentioned → should be allowed
	if !p.CanSpeak("agent1", true, now.Add(5*time.Second)) {
		t.Fatal("@mention should bypass cooldown")
	}
}

// ══════════════════════════════════════════════
// GroupChat integration tests
// ══════════════════════════════════════════════

func TestGroupChat_ProcessMessage_MentionReply(t *testing.T) {
	gc := setupGroupChat()

	msg := GroupMessage{
		SenderID:        "user1",
		SenderName:      "张三",
		Content:         "你好呀",
		MentionedAgents: []string{"linwanqing"},
		Timestamp:       time.Now(),
	}

	reply := gc.ProcessMessage(context.Background(), msg)
	if reply == nil {
		t.Fatal("expected reply when @mentioned")
	}
	if reply.AgentID != "linwanqing" {
		t.Fatalf("expected linwanqing, got %s", reply.AgentID)
	}
	if reply.Reason != "mention" {
		t.Fatalf("expected reason=mention, got %s", reply.Reason)
	}
	if reply.Content == "" {
		t.Fatal("expected non-empty content")
	}
}

func TestGroupChat_ProcessMessage_Silent(t *testing.T) {
	gc := setupGroupChat()
	// Override both agents to have 0 talkativeness
	gc.Registry.Get("linwanqing").Card.Talkativeness = 0.0
	gc.Registry.Get("fortuneteller").Card.Talkativeness = 0.0

	silentCount := 0
	for i := 0; i < 10; i++ {
		msg := GroupMessage{
			SenderID:   "user1",
			SenderName: "张三",
			Content:    "random unrelated message",
			Timestamp:  time.Now(),
		}
		reply := gc.ProcessMessage(context.Background(), msg)
		if reply == nil {
			silentCount++
		}
	}
	if silentCount < 8 {
		t.Fatalf("expected mostly silent with 0 talkativeness, got silent %d/10", silentCount)
	}
}

func TestGroupChat_SharedHistory_Visible(t *testing.T) {
	gc := setupGroupChat()

	// Send 3 messages
	for i := 0; i < 3; i++ {
		gc.ProcessMessage(context.Background(), GroupMessage{
			SenderID:   "user1",
			SenderName: "张三",
			Content:    fmt.Sprintf("消息 %d", i),
			Timestamp:  time.Now(),
		})
	}

	// SharedContext should have messages (user msgs + any agent replies)
	if gc.Context.Len() < 3 {
		t.Fatalf("expected at least 3 messages in shared history, got %d", gc.Context.Len())
	}
}

func TestGroupChat_Introduction_Injected(t *testing.T) {
	gc := setupGroupChat()

	// First message triggers introduction injection
	gc.ProcessMessage(context.Background(), GroupMessage{
		SenderID:        "user1",
		SenderName:      "张三",
		Content:         "大家好",
		MentionedAgents: []string{"linwanqing"},
		Timestamp:       time.Now(),
	})

	// Check that introductions were injected into agent's system prompt
	agent := gc.agents["linwanqing"]
	if agent == nil || agent.loop == nil {
		t.Fatal("agent not found")
	}
	if !contains_str_in(agent.loop.SystemPrompt, "群聊成员") {
		t.Fatal("expected introduction in system prompt")
	}
	if !contains_str_in(agent.loop.SystemPrompt, "运势大师") {
		t.Fatal("expected other agent mentioned in introduction")
	}
}

// ══════════════════════════════════════════════
// SharedContext tests
// ══════════════════════════════════════════════

func TestSharedContext_Append_Trim(t *testing.T) {
	sc := NewSharedContext(5) // max 5 messages

	for i := 0; i < 10; i++ {
		sc.Append(GroupMessage{
			SenderID:   "u1",
			SenderName: "张三",
			Content:    fmt.Sprintf("msg %d", i),
		})
	}

	if sc.Len() != 5 {
		t.Fatalf("expected 5 messages after trim, got %d", sc.Len())
	}
}

func TestSharedContext_Format(t *testing.T) {
	sc := NewSharedContext()
	sc.Append(GroupMessage{SenderName: "张三", Content: "你好"})
	sc.AppendReply(GroupReply{AgentName: "林晚晴", Content: "嗨～"})

	history := sc.GetHistory()
	if len(history) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(history))
	}
	if history[0]["role"] != "user" {
		t.Fatal("first message should be user")
	}
	if history[1]["role"] != "assistant" {
		t.Fatal("second message should be assistant")
	}
}

// helper
func contains_str_in(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
