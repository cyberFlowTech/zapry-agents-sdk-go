package agentsdk

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// ──────────────────────────────────────────────
// Loop Detector — detect repetitive tool call patterns
// ──────────────────────────────────────────────

// LoopDetectorConfig controls loop detection behavior.
type LoopDetectorConfig struct {
	Enabled             bool
	MaxRepeatCalls      int // consecutive same tool+args limit, default 3
	MaxSameToolInWindow int // same tool count limit within window, default 5
	WindowSize          int // sliding window size, default 10
}

// DefaultLoopDetectorConfig returns sensible defaults.
func DefaultLoopDetectorConfig() LoopDetectorConfig {
	return LoopDetectorConfig{
		Enabled:             true,
		MaxRepeatCalls:      3,
		MaxSameToolInWindow: 5,
		WindowSize:          10,
	}
}

// LoopWarning describes a detected loop pattern.
type LoopWarning struct {
	Type    string // "repeat" / "flood" / "ping_pong"
	Message string
}

type toolCallEntry struct {
	Name     string
	ArgsHash string
}

// LoopDetector tracks recent tool calls and detects repetitive patterns.
type LoopDetector struct {
	config  LoopDetectorConfig
	history []toolCallEntry
}

// NewLoopDetector creates a detector with the given config.
func NewLoopDetector(config ...LoopDetectorConfig) *LoopDetector {
	cfg := DefaultLoopDetectorConfig()
	if len(config) > 0 {
		cfg = config[0]
	}
	return &LoopDetector{config: cfg}
}

// Check analyzes whether the next call would trigger a loop warning.
// Returns nil if no issue detected.
func (d *LoopDetector) Check(name string, args map[string]interface{}) *LoopWarning {
	if !d.config.Enabled {
		return nil
	}

	hash := hashArgs(args)
	entry := toolCallEntry{Name: name, ArgsHash: hash}

	// 1. genericRepeat: consecutive same tool + same args
	if d.config.MaxRepeatCalls > 0 {
		repeatCount := 0
		for i := len(d.history) - 1; i >= 0; i-- {
			if d.history[i].Name == entry.Name && d.history[i].ArgsHash == entry.ArgsHash {
				repeatCount++
			} else {
				break
			}
		}
		if repeatCount >= d.config.MaxRepeatCalls {
			return &LoopWarning{
				Type:    "repeat",
				Message: fmt.Sprintf("Tool %q called %d times with identical arguments", name, repeatCount+1),
			}
		}
	}

	// 2. sameToolFlood: too many calls to same tool in window
	if d.config.MaxSameToolInWindow > 0 {
		windowStart := len(d.history) - d.config.WindowSize
		if windowStart < 0 {
			windowStart = 0
		}
		sameCount := 0
		for i := windowStart; i < len(d.history); i++ {
			if d.history[i].Name == name {
				sameCount++
			}
		}
		if sameCount >= d.config.MaxSameToolInWindow {
			return &LoopWarning{
				Type:    "flood",
				Message: fmt.Sprintf("Tool %q called %d times in last %d calls", name, sameCount+1, d.config.WindowSize),
			}
		}
	}

	// 3. pingPong: A/B/A/B alternating pattern (check last 3 entries)
	if len(d.history) >= 3 {
		h := d.history
		n := len(h)
		if h[n-1].Name != h[n-2].Name &&
			h[n-1].Name == h[n-3].Name &&
			h[n-2].Name == name &&
			name != h[n-1].Name {
			return &LoopWarning{
				Type:    "ping_pong",
				Message: fmt.Sprintf("Ping-pong pattern detected: %s / %s alternating", h[n-1].Name, name),
			}
		}
	}

	return nil
}

// Record adds a tool call to the history.
func (d *LoopDetector) Record(name string, args map[string]interface{}) {
	d.history = append(d.history, toolCallEntry{
		Name:     name,
		ArgsHash: hashArgs(args),
	})
	// Trim to 2x window to avoid unbounded growth
	maxKeep := d.config.WindowSize * 2
	if maxKeep < 20 {
		maxKeep = 20
	}
	if len(d.history) > maxKeep {
		d.history = d.history[len(d.history)-maxKeep:]
	}
}

// Reset clears the history.
func (d *LoopDetector) Reset() {
	d.history = nil
}

func hashArgs(args map[string]interface{}) string {
	if args == nil || len(args) == 0 {
		return "empty"
	}
	data, _ := json.Marshal(args)
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:8])
}
