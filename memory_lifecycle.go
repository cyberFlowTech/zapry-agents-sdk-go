package agentsdk

import (
	"context"
	"log"
	"math"
	"time"
)

// ──────────────────────────────────────────────
// Importance Scoring + Decay + Lifecycle Management
// ──────────────────────────────────────────────

// ImportanceScorer computes the importance of a memory entry,
// taking into account the current conversation context.
type ImportanceScorer interface {
	Score(memory TypedMemory, state *ConversationState) float64
}

// DefaultImportanceScorer combines recency, access frequency,
// and memory type to produce a 0–1 importance score.
type DefaultImportanceScorer struct {
	RecencyWeight   float64 // weight for time-based recency, default 0.4
	FrequencyWeight float64 // weight for access count, default 0.3
	TypeWeight      float64 // weight for memory type, default 0.3
}

// NewDefaultImportanceScorer creates a scorer with sensible defaults.
func NewDefaultImportanceScorer() *DefaultImportanceScorer {
	return &DefaultImportanceScorer{
		RecencyWeight:   0.4,
		FrequencyWeight: 0.3,
		TypeWeight:      0.3,
	}
}

func (s *DefaultImportanceScorer) Score(memory TypedMemory, state *ConversationState) float64 {
	recency := recencyScore(memory.UpdatedAt)
	frequency := frequencyScore(memory.AccessCnt)
	typeScore := memoryTypeScore(memory.Type)

	total := s.RecencyWeight*recency + s.FrequencyWeight*frequency + s.TypeWeight*typeScore
	return clamp01(total)
}

func recencyScore(updatedAt time.Time) float64 {
	days := time.Since(updatedAt).Hours() / 24.0
	// Exponential decay with 30-day half-life
	return math.Exp(-0.693 * days / 30.0)
}

func frequencyScore(accessCnt int) float64 {
	// Logarithmic scaling, diminishing returns above 10 accesses
	if accessCnt <= 0 {
		return 0
	}
	return math.Min(1.0, math.Log2(float64(accessCnt)+1)/math.Log2(11))
}

func memoryTypeScore(mt MemoryType) float64 {
	switch mt {
	case MemoryTypeSemantic:
		return 0.7
	case MemoryTypeProcedural:
		return 0.9
	case MemoryTypeEpisodic:
		return 0.5
	default:
		return 0.5
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// ──────────────────────────────────────────────
// Decay Policy — time-based forgetting
// ──────────────────────────────────────────────

// DecayPolicy controls how memory scores decay over time
// and when memories should be pruned.
type DecayPolicy struct {
	HalfLife      time.Duration // half-life for exponential decay, default 30 days
	MinScore      float64       // memories below this score are eligible for removal
	BoostOnAccess float64       // score boost when a memory is retrieved, default 0.1
}

// DefaultDecayPolicy returns production defaults.
func DefaultDecayPolicy() DecayPolicy {
	return DecayPolicy{
		HalfLife:      30 * 24 * time.Hour,
		MinScore:      0.05,
		BoostOnAccess: 0.1,
	}
}

// DecayScore applies exponential time-decay to a memory's score.
func (d DecayPolicy) DecayScore(original float64, lastUpdated time.Time) float64 {
	elapsed := time.Since(lastUpdated)
	halfLifeHours := d.HalfLife.Hours()
	if halfLifeHours <= 0 {
		return original
	}
	decayed := original * math.Exp(-0.693*elapsed.Hours()/halfLifeHours)
	return decayed
}

// ShouldPrune returns true if the decayed score is below the minimum threshold.
func (d DecayPolicy) ShouldPrune(decayedScore float64) bool {
	return decayedScore < d.MinScore
}

// Boost adds a score boost (clamped to 1.0) after a memory is accessed.
func (d DecayPolicy) Boost(currentScore float64) float64 {
	return clamp01(currentScore + d.BoostOnAccess)
}

// ──────────────────────────────────────────────
// Memory Lifecycle Manager — pruning, decay, GDPR
// ──────────────────────────────────────────────

// MemoryLifecycleManager handles periodic maintenance of the memory system:
// score decay, pruning of low-importance memories, and GDPR deletion.
type MemoryLifecycleManager struct {
	typed  *TypedMemoryStore
	scorer ImportanceScorer
	decay  DecayPolicy

	// Optional: vector store for cascading deletes
	semantic *SemanticMemoryStore
}

// NewMemoryLifecycleManager creates a lifecycle manager.
func NewMemoryLifecycleManager(typed *TypedMemoryStore, scorer ImportanceScorer, decay DecayPolicy) *MemoryLifecycleManager {
	if scorer == nil {
		scorer = NewDefaultImportanceScorer()
	}
	return &MemoryLifecycleManager{
		typed:  typed,
		scorer: scorer,
		decay:  decay,
	}
}

// SetSemanticStore enables cascading deletes to the vector store.
func (m *MemoryLifecycleManager) SetSemanticStore(sem *SemanticMemoryStore) {
	m.semantic = sem
}

// RunDecayCycle applies decay to all memories and prunes those below threshold.
// Returns the number of pruned memories.
func (m *MemoryLifecycleManager) RunDecayCycle(ctx context.Context) (int, error) {
	pruned := 0

	for _, mt := range []MemoryType{MemoryTypeSemantic, MemoryTypeEpisodic, MemoryTypeProcedural} {
		mems, err := m.typed.ListByType(mt)
		if err != nil {
			return pruned, err
		}

		for _, mem := range mems {
			decayed := m.decay.DecayScore(mem.Score, mem.UpdatedAt)

			if m.decay.ShouldPrune(decayed) {
				if err := m.typed.Remove(mt, mem.ID); err != nil {
					log.Printf("[MemoryLifecycle] Failed to prune %s/%s: %v", mt, mem.ID, err)
					continue
				}
				if m.semantic != nil {
					m.semantic.DeleteMemory(ctx, mem.ID, "", "")
				}
				pruned++
				continue
			}

			if decayed != mem.Score {
				mem.Score = decayed
				m.typed.Update(mem)
			}
		}
	}

	return pruned, nil
}

// ForgetUser removes all memories for a user namespace (GDPR right-to-forget).
// This clears typed memories, structured store data, and vector store entries.
func (m *MemoryLifecycleManager) ForgetUser(ctx context.Context, namespace string) error {
	if err := m.typed.ClearAll(); err != nil {
		return err
	}

	if m.semantic != nil {
		return m.semantic.DeleteNamespaceVectors(ctx, namespace)
	}
	return nil
}

// ──────────────────────────────────────────────
// Memory Audit Log
// ──────────────────────────────────────────────

// MemoryAuditAction represents the type of auditable memory operation.
type MemoryAuditAction string

const (
	AuditActionAdd    MemoryAuditAction = "add"
	AuditActionUpdate MemoryAuditAction = "update"
	AuditActionDelete MemoryAuditAction = "delete"
	AuditActionAccess MemoryAuditAction = "access"
	AuditActionPrune  MemoryAuditAction = "prune"
	AuditActionForget MemoryAuditAction = "forget"
)

// MemoryAuditEntry represents a single auditable memory event.
type MemoryAuditEntry struct {
	Timestamp time.Time         `json:"timestamp"`
	Action    MemoryAuditAction `json:"action"`
	Namespace string            `json:"namespace"`
	MemoryID  string            `json:"memory_id,omitempty"`
	Type      MemoryType        `json:"type,omitempty"`
	Details   string            `json:"details,omitempty"`
}

// MemoryAuditLogger receives audit events for external processing
// (e.g. writing to Kafka, ClickHouse, or a log file).
type MemoryAuditLogger interface {
	Log(entry MemoryAuditEntry)
}

// NoopAuditLogger discards all audit events. Used as default.
type NoopAuditLogger struct{}

func (NoopAuditLogger) Log(MemoryAuditEntry) {}
