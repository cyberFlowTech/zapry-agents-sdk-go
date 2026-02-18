package agentsdk

import (
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// GroupChat — Speaking Policy (cooldown + rate limiting)
// ──────────────────────────────────────────────

// SpeakingPolicy controls Agent speaking behavior in group chats.
type SpeakingPolicy struct {
	CooldownDuration time.Duration // agent cooldown after speaking, default 30s
	MaxAgentsPerMsg  int           // max agents that can reply to one message, default 1

	mu        sync.Mutex
	lastSpoke map[string]time.Time // agentID -> last speak time
	msgReplies map[string]int      // msgKey -> reply count (reset per message)
	currentMsg string
}

// DefaultSpeakingPolicy returns production defaults.
func DefaultSpeakingPolicy() *SpeakingPolicy {
	return &SpeakingPolicy{
		CooldownDuration: 30 * time.Second,
		MaxAgentsPerMsg:  1,
		lastSpoke:        make(map[string]time.Time),
		msgReplies:       make(map[string]int),
	}
}

// CanSpeak checks if an Agent is allowed to speak.
// isMentioned=true bypasses cooldown (must always respond to @mention).
func (p *SpeakingPolicy) CanSpeak(agentID string, isMentioned bool, now time.Time) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	// @mention bypasses cooldown
	if isMentioned {
		return true
	}

	// Check cooldown
	if last, ok := p.lastSpoke[agentID]; ok {
		if now.Sub(last) < p.CooldownDuration {
			return false
		}
	}

	// Check per-message limit
	if p.MaxAgentsPerMsg > 0 && p.msgReplies[p.currentMsg] >= p.MaxAgentsPerMsg {
		return false
	}

	return true
}

// RecordSpeak records that an Agent spoke.
func (p *SpeakingPolicy) RecordSpeak(agentID string, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastSpoke[agentID] = now
	p.msgReplies[p.currentMsg]++
}

// SetCurrentMessage sets the current message key for per-message reply counting.
func (p *SpeakingPolicy) SetCurrentMessage(msgKey string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.currentMsg = msgKey
	// Clean up old entries (keep last 100)
	if len(p.msgReplies) > 100 {
		p.msgReplies = make(map[string]int)
	}
}
