package persona

import (
	"time"

)

// SelectTodayEvent picks a daily event from the pool using daySeed.
// recentEventIDs contains event IDs from the past 3 days for dedup.
func SelectTodayEvent(pool []EventTemplate, personaID string, now time.Time, recentEventIDs []string) string {
	if len(pool) == 0 {
		return ""
	}

	// Build set of recent categories for dedup
	recentCategories := make(map[string]bool)
	for _, id := range recentEventIDs {
		// Extract category from event ID (e.g., "pet_3" → "pet")
		for i := range id {
			if id[i] == '_' {
				recentCategories[id[:i]] = true
				break
			}
		}
	}

	// Filter pool: exclude recently used categories
	filtered := make([]EventTemplate, 0, len(pool))
	for _, e := range pool {
		if !recentCategories[e.Category] {
			filtered = append(filtered, e)
		}
	}

	// Fallback to full pool if too few remain
	if len(filtered) < 3 {
		filtered = pool
	}

	// Use daySeed for deterministic daily selection
	seed := daySeed(personaID, now, "event")

	// Weighted random selection
	items := make([]string, len(filtered))
	weights := make([]float64, len(filtered))
	for i, e := range filtered {
		items[i] = e.Event
		weights[i] = e.Weight
	}

	return weightedRandom(items, weights, seed)
}
