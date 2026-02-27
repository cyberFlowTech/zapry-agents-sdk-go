package agentsdk

import (
	"testing"
	"time"
)

func TestDefaultImportanceScorer(t *testing.T) {
	scorer := NewDefaultImportanceScorer()

	recent := TypedMemory{
		Type:      MemoryTypeSemantic,
		UpdatedAt: time.Now(),
		AccessCnt: 5,
	}
	score := scorer.Score(recent, nil)
	if score < 0.5 {
		t.Errorf("recent + frequently accessed memory should score high, got %f", score)
	}

	old := TypedMemory{
		Type:      MemoryTypeEpisodic,
		UpdatedAt: time.Now().Add(-180 * 24 * time.Hour), // 6 months ago
		AccessCnt: 0,
	}
	oldScore := scorer.Score(old, nil)
	if oldScore >= score {
		t.Errorf("old unused memory should score lower: %f >= %f", oldScore, score)
	}
}

func TestDefaultImportanceScorer_TypeWeighting(t *testing.T) {
	scorer := NewDefaultImportanceScorer()
	now := time.Now()

	procedural := TypedMemory{Type: MemoryTypeProcedural, UpdatedAt: now, AccessCnt: 3}
	semantic := TypedMemory{Type: MemoryTypeSemantic, UpdatedAt: now, AccessCnt: 3}
	episodic := TypedMemory{Type: MemoryTypeEpisodic, UpdatedAt: now, AccessCnt: 3}

	ps := scorer.Score(procedural, nil)
	ss := scorer.Score(semantic, nil)
	es := scorer.Score(episodic, nil)

	if ps <= ss || ss <= es {
		t.Errorf("expected procedural > semantic > episodic: %f, %f, %f", ps, ss, es)
	}
}

func TestDecayPolicy_DecayScore(t *testing.T) {
	policy := DefaultDecayPolicy()

	score := policy.DecayScore(1.0, time.Now())
	if score < 0.99 {
		t.Errorf("just-updated memory should barely decay: %f", score)
	}

	halfLife := policy.DecayScore(1.0, time.Now().Add(-30*24*time.Hour))
	if halfLife > 0.55 || halfLife < 0.45 {
		t.Errorf("after one half-life, expected ~0.5, got %f", halfLife)
	}

	old := policy.DecayScore(1.0, time.Now().Add(-90*24*time.Hour))
	if old > 0.15 {
		t.Errorf("after 3 half-lives, expected < 0.15, got %f", old)
	}
}

func TestDecayPolicy_ShouldPrune(t *testing.T) {
	policy := DefaultDecayPolicy()

	if policy.ShouldPrune(0.5) {
		t.Error("0.5 should not be pruned")
	}
	if !policy.ShouldPrune(0.01) {
		t.Error("0.01 should be pruned (below 0.05 threshold)")
	}
}

func TestDecayPolicy_Boost(t *testing.T) {
	policy := DefaultDecayPolicy()

	boosted := policy.Boost(0.5)
	if boosted != 0.6 {
		t.Errorf("expected 0.6, got %f", boosted)
	}

	clamped := policy.Boost(0.95)
	if clamped != 1.0 {
		t.Errorf("expected clamped to 1.0, got %f", clamped)
	}
}

func TestMemoryLifecycleManager_RunDecayCycle(t *testing.T) {
	store := NewInMemoryMemoryStore()
	typed := NewTypedMemoryStore(store, "test:user1")

	typed.Add(TypedMemory{
		ID: "fresh", Type: MemoryTypeSemantic, Content: "fresh fact",
		Score: 0.8, UpdatedAt: time.Now(),
	})
	typed.Add(TypedMemory{
		ID: "stale", Type: MemoryTypeSemantic, Content: "stale fact",
		Score: 0.02, UpdatedAt: time.Now().Add(-365 * 24 * time.Hour),
	})

	mgr := NewMemoryLifecycleManager(typed, nil, DefaultDecayPolicy())
	pruned, err := mgr.RunDecayCycle(nil)
	if err != nil {
		t.Fatal(err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}

	remaining, _ := typed.ListByType(MemoryTypeSemantic)
	if len(remaining) != 1 || remaining[0].ID != "fresh" {
		t.Error("only fresh memory should remain")
	}
}

func TestRecencyScore(t *testing.T) {
	now := recencyScore(time.Now())
	if now < 0.99 {
		t.Errorf("just-now should be ~1.0, got %f", now)
	}

	monthAgo := recencyScore(time.Now().Add(-30 * 24 * time.Hour))
	if monthAgo > 0.55 || monthAgo < 0.45 {
		t.Errorf("30-day-old should be ~0.5, got %f", monthAgo)
	}
}

func TestFrequencyScore(t *testing.T) {
	if frequencyScore(0) != 0 {
		t.Error("0 accesses should score 0")
	}
	ten := frequencyScore(10)
	if ten < 0.95 {
		t.Errorf("10 accesses should score ~1.0, got %f", ten)
	}
	one := frequencyScore(1)
	if one >= ten {
		t.Error("1 access should score less than 10")
	}
}

func TestNoopAuditLogger(t *testing.T) {
	logger := NoopAuditLogger{}
	logger.Log(MemoryAuditEntry{Action: AuditActionAdd})
}
