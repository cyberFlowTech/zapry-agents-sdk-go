package agentsdk

import (
	"log"
	"sync"
)

// AgentRegistryStore is a permission-aware Agent registry.
type AgentRegistryStore struct {
	mu     sync.RWMutex
	agents map[string]*AgentRuntimeConfig
}

func NewAgentRegistry2() *AgentRegistryStore {
	return &AgentRegistryStore{agents: make(map[string]*AgentRuntimeConfig)}
}

func (r *AgentRegistryStore) Register(rt *AgentRuntimeConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[rt.Card.AgentID] = rt
	log.Printf("[AgentRegistry] Registered: %s", rt.Card.AgentID)
}

func (r *AgentRegistryStore) Get(agentID string) *AgentRuntimeConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.agents[agentID]
}

func (r *AgentRegistryStore) ListAll() []*AgentRuntimeConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]*AgentRuntimeConfig, 0, len(r.agents))
	for _, a := range r.agents {
		list = append(list, a)
	}
	return list
}

func (r *AgentRegistryStore) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.agents)
}

func (r *AgentRegistryStore) FindBySkill(skill, callerOwnerID, callerOrgID string) []*AgentRuntimeConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var results []*AgentRuntimeConfig
	for _, rt := range r.agents {
		if !contains_str(rt.Card.Skills, skill) {
			continue
		}
		if !isVisible(rt.Card, callerOwnerID, callerOrgID) {
			continue
		}
		results = append(results, rt)
	}
	return results
}

func (r *AgentRegistryStore) CanHandoff(fromAgent, toAgent, callerOwnerID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	target, ok := r.agents[toAgent]
	if !ok || target.Card.HandoffPolicyStr == "deny" {
		return false
	}
	return isVisible(target.Card, callerOwnerID, "")
}

func isVisible(card AgentCardPublic, callerOwnerID, callerOrgID string) bool {
	switch card.Visibility {
	case "public":
		return true
	case "org":
		return card.OrgID != "" && callerOrgID != "" && card.OrgID == callerOrgID
	default: // private
		return callerOwnerID != "" && card.OwnerID == callerOwnerID
	}
}
