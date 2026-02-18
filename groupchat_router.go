package agentsdk

import (
	"math/rand"
	"strings"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// GroupChat — Router (4-layer message routing)
// ──────────────────────────────────────────────

// RouteResult describes which Agent should respond and why.
type RouteResult struct {
	AgentID string
	Reason  string // "mention" / "skill" / "followup" / "talkativeness" / "none"
}

// GroupChatRouter determines which Agent should respond to a group message.
// Uses 4-layer routing: @mention → skill match → followup detection → talkativeness probability.
type GroupChatRouter struct {
	registry *AgentRegistryStore
	rng      *rand.Rand
	rngMu    sync.Mutex
}

// NewGroupChatRouter creates a router backed by the given agent registry.
func NewGroupChatRouter(registry *AgentRegistryStore) *GroupChatRouter {
	return &GroupChatRouter{
		registry: registry,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Route determines which Agent should respond to the given message.
// agents: list of Agent IDs currently in the group chat.
// lastAgentReply: agentID of the last Agent that spoke (for followup detection).
// lastReplyTime: when the last Agent reply was sent.
// Returns nil if no Agent should respond (silence).
func (r *GroupChatRouter) Route(
	msg GroupMessage,
	agents []string,
	lastAgentReply string,
	lastReplyTime time.Time,
) *RouteResult {
	// Layer 1: @mention — highest priority
	if len(msg.MentionedAgents) > 0 {
		for _, mentioned := range msg.MentionedAgents {
			for _, agentID := range agents {
				if agentID == mentioned {
					return &RouteResult{AgentID: agentID, Reason: "mention"}
				}
			}
		}
	}

	// Layer 2: skill match — check if message content matches any agent's skills
	if result := r.matchSkill(msg.Content, agents); result != nil {
		return result
	}

	// Layer 3: followup — if user is replying to a recent agent message
	if lastAgentReply != "" && !lastReplyTime.IsZero() {
		if time.Since(lastReplyTime) < 60*time.Second {
			for _, agentID := range agents {
				if agentID == lastAgentReply {
					return &RouteResult{AgentID: agentID, Reason: "followup"}
				}
			}
		}
	}

	// Layer 4: talkativeness probability
	return r.talkativenessRoute(agents)
}

// matchSkill checks if the message content matches any agent's skills.
func (r *GroupChatRouter) matchSkill(content string, agents []string) *RouteResult {
	lower := strings.ToLower(content)
	for _, agentID := range agents {
		rt := r.registry.Get(agentID)
		if rt == nil {
			continue
		}
		for _, skill := range rt.Card.Skills {
			if strings.Contains(lower, strings.ToLower(skill)) {
				return &RouteResult{AgentID: agentID, Reason: "skill"}
			}
		}
	}
	return nil
}

// talkativenessRoute uses probability based on each agent's Talkativeness setting.
func (r *GroupChatRouter) talkativenessRoute(agents []string) *RouteResult {
	type candidate struct {
		agentID       string
		talkativeness float64
	}

	var candidates []candidate
	for _, agentID := range agents {
		rt := r.registry.Get(agentID)
		if rt == nil {
			continue
		}
		t := rt.Card.Talkativeness
		if t <= 0 {
			continue
		}

		r.rngMu.Lock()
		roll := r.rng.Float64()
		r.rngMu.Unlock()

		if roll < t {
			candidates = append(candidates, candidate{agentID: agentID, talkativeness: t})
		}
	}

	if len(candidates) == 0 {
		return nil // silence
	}

	// Pick the one with highest talkativeness
	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.talkativeness > best.talkativeness {
			best = c
		}
	}

	return &RouteResult{AgentID: best.agentID, Reason: "talkativeness"}
}
