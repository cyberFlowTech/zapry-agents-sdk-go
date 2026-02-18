package agentsdk

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ──────────────────────────────────────────────
// GroupChat — Multi-Agent group conversation manager
// ──────────────────────────────────────────────

// groupAgent holds per-agent state within a group chat.
type groupAgent struct {
	agentID string
	session *MemorySession
	nc      *NaturalConversation
	loop    *AgentLoop
}

// GroupChat manages a multi-agent conversation in a shared chat room.
// Multiple Agents and humans share a message history; the router decides
// which Agent responds to each message.
type GroupChat struct {
	Registry *AgentRegistryStore
	Router   *GroupChatRouter
	Policy   *SpeakingPolicy
	Context  *SharedContext

	agents         map[string]*groupAgent // agentID -> per-agent state
	agentOrder     []string               // ordered list of agent IDs
	introSent      bool
	lastAgentReply string
	lastReplyTime  time.Time
}

// NewGroupChat creates a group chat backed by the given agent registry.
func NewGroupChat(registry *AgentRegistryStore) *GroupChat {
	return &GroupChat{
		Registry: registry,
		Router:   NewGroupChatRouter(registry),
		Policy:   DefaultSpeakingPolicy(),
		Context:  NewSharedContext(),
		agents:   make(map[string]*groupAgent),
	}
}

// AddAgent adds an Agent to this group chat.
// The agent must already be registered in the AgentRegistryStore.
// session: the agent's independent MemorySession.
// nc: the agent's NaturalConversation config (can be nil for basic mode).
func (gc *GroupChat) AddAgent(agentID string, session *MemorySession, nc *NaturalConversation) {
	rt := gc.Registry.Get(agentID)
	if rt == nil {
		return
	}
	loop := NewAgentLoop(rt.LLMFn, rt.ToolReg, rt.SystemPrompt, rt.MaxTurns, nil)
	if rt.Guardrails != nil {
		loop.Guardrails = rt.Guardrails
	}
	if rt.Tracer != nil {
		loop.Tracer = rt.Tracer
	}

	gc.agents[agentID] = &groupAgent{
		agentID: agentID,
		session: session,
		nc:      nc,
		loop:    loop,
	}
	gc.agentOrder = append(gc.agentOrder, agentID)
}

// RemoveAgent removes an Agent from this group chat.
func (gc *GroupChat) RemoveAgent(agentID string) {
	delete(gc.agents, agentID)
	for i, id := range gc.agentOrder {
		if id == agentID {
			gc.agentOrder = append(gc.agentOrder[:i], gc.agentOrder[i+1:]...)
			break
		}
	}
}

// ListAgents returns the IDs of all Agents in this group chat.
func (gc *GroupChat) ListAgents() []string {
	result := make([]string, len(gc.agentOrder))
	copy(result, gc.agentOrder)
	return result
}

// ProcessMessage handles an incoming group message.
// Returns a GroupReply if an Agent responds, or nil if all Agents stay silent.
func (gc *GroupChat) ProcessMessage(ctx context.Context, msg GroupMessage) *GroupReply {
	now := time.Now()
	if msg.Timestamp.IsZero() {
		msg.Timestamp = now
	}

	// Skip messages from Agents (don't respond to other agents in v1)
	if msg.IsFromAgent {
		gc.Context.Append(msg)
		return nil
	}

	// Inject introductions on first message
	if !gc.introSent && len(gc.agents) > 0 {
		gc.injectIntroductions()
		gc.introSent = true
	}

	// Add message to shared history
	gc.Context.Append(msg)

	// Set current message for policy tracking
	msgKey := fmt.Sprintf("%s:%d", msg.SenderID, msg.Timestamp.UnixNano())
	gc.Policy.SetCurrentMessage(msgKey)

	// Route: determine which Agent should respond
	routeResult := gc.Router.Route(msg, gc.agentOrder, gc.lastAgentReply, gc.lastReplyTime)
	if routeResult == nil {
		return nil // silence
	}

	// Check speaking policy
	isMentioned := routeResult.Reason == "mention"
	if !gc.Policy.CanSpeak(routeResult.AgentID, isMentioned, now) {
		return nil
	}

	// Get agent state
	agent, ok := gc.agents[routeResult.AgentID]
	if !ok {
		return nil
	}

	// Get agent display name
	rt := gc.Registry.Get(routeResult.AgentID)
	displayName := routeResult.AgentID
	if rt != nil {
		if rt.Card.DisplayName != "" {
			displayName = rt.Card.DisplayName
		} else if rt.Card.Name != "" {
			displayName = rt.Card.Name
		}
	}

	// Execute agent
	var result *AgentLoopResult
	sharedHistory := gc.Context.GetHistory()

	if agent.nc != nil {
		naturalLoop := agent.nc.WrapLoop(agent.loop)
		result = naturalLoop.RunContext(ctx, agent.session, msg.Content, sharedHistory)
	} else {
		result = agent.loop.RunContext(ctx, msg.Content, sharedHistory, "")
	}

	if result == nil || result.FinalOutput == "" {
		return nil
	}

	// Record
	gc.Policy.RecordSpeak(routeResult.AgentID, now)
	gc.lastAgentReply = routeResult.AgentID
	gc.lastReplyTime = now

	reply := &GroupReply{
		AgentID:   routeResult.AgentID,
		AgentName: displayName,
		Content:   result.FinalOutput,
		Reason:    routeResult.Reason,
	}

	gc.Context.AppendReply(*reply)

	return reply
}

// injectIntroductions sends a group member introduction to each agent's system prompt.
func (gc *GroupChat) injectIntroductions() {
	intro := gc.buildIntroduction()
	for _, agent := range gc.agents {
		// Inject into the agent's loop system prompt
		if agent.loop.SystemPrompt != "" {
			agent.loop.SystemPrompt = agent.loop.SystemPrompt + "\n\n" + intro
		} else {
			agent.loop.SystemPrompt = intro
		}
	}
}

// buildIntroduction generates the group member introduction text.
func (gc *GroupChat) buildIntroduction() string {
	var lines []string
	lines = append(lines, "[群聊成员]")

	for _, agentID := range gc.agentOrder {
		rt := gc.Registry.Get(agentID)
		if rt == nil {
			continue
		}
		name := rt.Card.DisplayName
		if name == "" {
			name = rt.Card.Name
		}
		desc := rt.Card.Description
		skills := ""
		if len(rt.Card.Skills) > 0 {
			skills = "，擅长" + strings.Join(rt.Card.Skills, "、")
		}
		lines = append(lines, fmt.Sprintf("- %s（AI）：%s%s", name, desc, skills))
	}

	lines = append(lines, "")
	lines = append(lines, "在群聊中请注意：")
	lines = append(lines, "- 只回答属于你领域的问题")
	lines = append(lines, "- 其他 Agent 已经在回复的话题不要重复回答")
	lines = append(lines, "- 不要每条消息都回复，保持自然节奏")
	lines = append(lines, "- 不要说「有什么可以帮你的」这类套话")

	return strings.Join(lines, "\n")
}
