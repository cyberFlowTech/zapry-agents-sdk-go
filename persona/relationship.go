package persona

import "time"

// RelationshipState tracks the evolving relationship between persona and user.
type RelationshipState struct {
	Closeness           int       `json:"closeness"`            // 0-100, default 10
	InteractionCount    int       `json:"interaction_count"`
	LastInteractionTime time.Time `json:"last_interaction_time"`
	Milestones          []string  `json:"milestones"`
	RecentSignals       []Signal  `json:"recent_signals"` // sliding window, max 20
}

// Signal represents a detected user emotional signal.
type Signal struct {
	Type      string    `json:"type"`      // positive|negative|neutral
	Detail    string    `json:"detail"`    // user_laughed|user_angry|topic_shared
	Timestamp time.Time `json:"timestamp"`
}

// DefaultRelationshipState returns the initial relationship state
// for a first-time interaction.
func DefaultRelationshipState() *RelationshipState {
	return &RelationshipState{
		Closeness:        10,
		InteractionCount: 0,
		Milestones:       []string{},
		RecentSignals:    []Signal{},
	}
}

// RelationshipUpdater is the contract for updating relationship state.
// v1: defined but not implemented (read-only).
// v1.5: auto-update after each turn.
type RelationshipUpdater interface {
	UpdateAfterTurn(state *RelationshipState, outcome TurnOutcome) *RelationshipState
}

// TurnOutcome captures the result of a conversation turn.
type TurnOutcome struct {
	Sentiment string   `json:"sentiment"` // positive|negative|neutral
	Signals   []Signal `json:"signals"`
}
