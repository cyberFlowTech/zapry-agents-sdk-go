package agentsdk

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// HealthCheckFunc checks one dependency/component health.
type HealthCheckFunc func(ctx context.Context) error

// HealthCheckResult is one check execution result.
type HealthCheckResult struct {
	Name       string  `json:"name"`
	Status     string  `json:"status"` // "ok" | "error"
	Error      string  `json:"error,omitempty"`
	DurationMs float64 `json:"duration_ms"`
}

// HealthReport aggregates all checks.
type HealthReport struct {
	Status    string              `json:"status"` // "ok" | "error"
	Timestamp string              `json:"timestamp"`
	Checks    []HealthCheckResult `json:"checks"`
}

// HealthChecker manages health checks and exposes HTTP endpoint.
type HealthChecker struct {
	mu     sync.RWMutex
	checks map[string]HealthCheckFunc
}

// NewHealthChecker creates an empty checker.
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]HealthCheckFunc),
	}
}

// RegisterCheck registers one named check.
func (h *HealthChecker) RegisterCheck(name string, check HealthCheckFunc) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("health check name is empty")
	}
	if check == nil {
		return errors.New("health check func is nil")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = check
	return nil
}

// RemoveCheck removes one check by name.
func (h *HealthChecker) RemoveCheck(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.checks, strings.TrimSpace(name))
}

// Check executes all checks and returns aggregate status.
func (h *HealthChecker) Check(ctx context.Context) HealthReport {
	h.mu.RLock()
	snapshot := make(map[string]HealthCheckFunc, len(h.checks))
	for name, fn := range h.checks {
		snapshot[name] = fn
	}
	h.mu.RUnlock()

	report := HealthReport{
		Status:    "ok",
		Timestamp: time.Now().Format(time.RFC3339),
		Checks:    make([]HealthCheckResult, 0, len(snapshot)),
	}

	for name, check := range snapshot {
		start := time.Now()
		err := check(ctx)
		result := HealthCheckResult{
			Name:       name,
			Status:     "ok",
			DurationMs: float64(time.Since(start).Microseconds()) / 1000.0,
		}
		if err != nil {
			result.Status = "error"
			result.Error = err.Error()
			report.Status = "error"
		}
		report.Checks = append(report.Checks, result)
	}

	return report
}

// ServeHTTP serves /healthz-style JSON response.
// - GET: returns check report (200 if healthy, 503 otherwise)
// - Others: 405
func (h *HealthChecker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "error",
			"error":  "method not allowed",
		})
		return
	}

	report := h.Check(r.Context())
	if report.Status != "ok" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(report)
}
