package persona

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

)

// ResolveState determines the current activity and energy based on time slots.
func ResolveState(config *RuntimeConfig, now time.Time) *CurrentState {
	hour := now.Hour()
	minute := now.Minute()
	timeMinutes := hour*60 + minute

	// Find matching slot
	var matchedSlot *TimeSlot
	for i := range config.StateMachine.Slots {
		slot := &config.StateMachine.Slots[i]
		start, end := parseSlotRange(slot.Range)
		if start <= end {
			if timeMinutes >= start && timeMinutes < end {
				matchedSlot = slot
				break
			}
		} else {
			// Wraps midnight: e.g., 22:00-06:00
			if timeMinutes >= start || timeMinutes < end {
				matchedSlot = slot
				break
			}
		}
	}

	// Fallback
	if matchedSlot == nil || len(matchedSlot.Activities) == 0 {
		return &CurrentState{Activity: "rest", Energy: 50}
	}

	// daySeed: ensures same activity within same day+slot
	seed := daySeed(config.PersonaID, now, matchedSlot.Range)
	activity := weightedRandom(matchedSlot.Activities, matchedSlot.Weights, seed)

	// Energy based on time of day
	energy := calculateEnergy(config.MoodModel.EnergyCurve, hour)

	return &CurrentState{
		Activity: activity,
		Energy:   energy,
	}
}

// daySeed generates a deterministic seed from persona_id + date + slot.
func daySeed(personaID string, now time.Time, slotRange string) int64 {
	dateStr := now.Format("2006-01-02")
	key := fmt.Sprintf("%s:%s:%s", personaID, dateStr, slotRange)
	h := sha256.Sum256([]byte(key))
	return int64(binary.BigEndian.Uint64(h[:8]))
}

// weightedRandom selects an item from the list using weights and a seed.
func weightedRandom(items []string, weights []float64, seed int64) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}

	r := rand.New(rand.NewSource(seed))

	// Normalize weights
	total := 0.0
	for _, w := range weights {
		total += w
	}
	if total == 0 {
		return items[r.Intn(len(items))]
	}

	roll := r.Float64() * total
	cumulative := 0.0
	for i, w := range weights {
		cumulative += w
		if roll <= cumulative {
			return items[i]
		}
	}
	return items[len(items)-1]
}

// calculateEnergy returns energy level (0-100) based on time and curve type.
func calculateEnergy(curve string, hour int) int {
	switch curve {
	case "night_low":
		// Peak at 10-14, low at 22-06
		switch {
		case hour >= 6 && hour < 10:
			return 60 + (hour-6)*5
		case hour >= 10 && hour < 14:
			return 80
		case hour >= 14 && hour < 18:
			return 70
		case hour >= 18 && hour < 22:
			return 50 - (hour-18)*5
		default: // 22-06
			return 25
		}
	case "morning_high":
		switch {
		case hour >= 5 && hour < 10:
			return 90
		case hour >= 10 && hour < 14:
			return 70
		case hour >= 14 && hour < 20:
			return 50
		default:
			return 30
		}
	default: // flat
		return 60
	}
}

// parseSlotRange parses "HH:MM-HH:MM" into total minutes.
func parseSlotRange(rangeStr string) (int, int) {
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return 0, 0
	}
	start := parseTime(parts[0])
	end := parseTime(parts[1])
	return start, end
}

func parseTime(t string) int {
	parts := strings.Split(strings.TrimSpace(t), ":")
	if len(parts) != 2 {
		return 0
	}
	h, _ := strconv.Atoi(parts[0])
	m, _ := strconv.Atoi(parts[1])
	return h*60 + m
}
