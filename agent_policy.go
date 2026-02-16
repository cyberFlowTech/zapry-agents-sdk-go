package agentsdk

import (
	"fmt"
	"sync"
	"time"
)

// HandoffPolicyConfig defines permission and loop guard rules.
type HandoffPolicyConfig struct {
	MaxHopCount     int
	DefaultTimeout  int  // ms
	AllowCrossOwner bool
}

// NewHandoffPolicy creates a policy with defaults.
func NewHandoffPolicy() *HandoffPolicyConfig {
	return &HandoffPolicyConfig{MaxHopCount: 3, DefaultTimeout: 30000}
}

// CheckAccess runs the permission pipeline. Returns nil if allowed.
func (p *HandoffPolicyConfig) CheckAccess(req *HandoffRequestData, target *AgentCardPublic) *HandoffErrorData {
	if target.HandoffPolicyStr == "deny" {
		return &HandoffErrorData{Code: "NOT_ALLOWED", Message: fmt.Sprintf("Agent %s denies handoff", target.AgentID)}
	}
	if target.SafetyLevel == "high" && req.RequestedMode == "tool_based" {
		return &HandoffErrorData{Code: "SAFETY_BLOCK", Message: "High safety agent requires coordinator mode"}
	}
	if target.HandoffPolicyStr == "coordinator_only" && req.RequestedMode == "tool_based" {
		return &HandoffErrorData{Code: "NOT_ALLOWED", Message: "Agent only accepts coordinator handoff"}
	}
	if target.Visibility == "private" && req.CallerOwnerID != target.OwnerID {
		return &HandoffErrorData{Code: "NOT_ALLOWED", Message: "Private agent: owner mismatch"}
	}
	if target.Visibility == "org" && (target.OrgID == "" || req.CallerOrgID != target.OrgID) {
		return &HandoffErrorData{Code: "NOT_ALLOWED", Message: "Org agent: org_id mismatch"}
	}
	if len(target.AllowedCallerAgents) > 0 && !contains_str(target.AllowedCallerAgents, req.FromAgent) {
		return &HandoffErrorData{Code: "NOT_ALLOWED", Message: "Caller agent not in whitelist"}
	}
	if len(target.AllowedCallerOwners) > 0 && !contains_str(target.AllowedCallerOwners, req.CallerOwnerID) {
		return &HandoffErrorData{Code: "NOT_ALLOWED", Message: "Caller owner not in whitelist"}
	}
	if !p.AllowCrossOwner && req.CallerOwnerID != "" && target.OwnerID != "" && req.CallerOwnerID != target.OwnerID {
		return &HandoffErrorData{Code: "NOT_ALLOWED", Message: "Cross-owner handoff disabled"}
	}
	return nil
}

// CheckLoop validates loop guard (based on next-hop state).
func (p *HandoffPolicyConfig) CheckLoop(req *HandoffRequestData) *HandoffErrorData {
	nextHop := req.HopCount + 1
	if nextHop > p.MaxHopCount {
		return &HandoffErrorData{Code: "LOOP_DETECTED", Message: fmt.Sprintf("Max hop count exceeded: %d > %d", nextHop, p.MaxHopCount)}
	}
	if contains_str(req.VisitedAgents, req.ToAgent) {
		return &HandoffErrorData{Code: "LOOP_DETECTED", Message: fmt.Sprintf("Agent %s already visited", req.ToAgent)}
	}
	return nil
}

func contains_str(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// HandoffIdempotencyCache provides request_id based caching.
type HandoffIdempotencyCache struct {
	mu    sync.Mutex
	cache map[string]*idempEntry
	ttl   time.Duration
}

type idempEntry struct {
	result *HandoffResultData
	ts     time.Time
}

func NewHandoffIdempotencyCache(ttl time.Duration) *HandoffIdempotencyCache {
	return &HandoffIdempotencyCache{cache: make(map[string]*idempEntry), ttl: ttl}
}

func (c *HandoffIdempotencyCache) GetOrSet(requestID string, result *HandoffResultData) (*HandoffResultData, bool) {
	if requestID == "" {
		return result, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cleanup()
	if e, ok := c.cache[requestID]; ok {
		cached := *e.result
		cached.CacheHit = true
		return &cached, true
	}
	c.cache[requestID] = &idempEntry{result: result, ts: time.Now()}
	return result, false
}

func (c *HandoffIdempotencyCache) cleanup() {
	now := time.Now()
	for k, e := range c.cache {
		if now.Sub(e.ts) > c.ttl {
			delete(c.cache, k)
		}
	}
}
