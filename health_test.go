package agentsdk

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthChecker_Check_AllOK(t *testing.T) {
	h := NewHealthChecker()
	if err := h.RegisterCheck("redis", func(ctx context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := h.RegisterCheck("mcp", func(ctx context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}

	report := h.Check(context.Background())
	if report.Status != "ok" {
		t.Fatalf("expected ok, got %s", report.Status)
	}
	if len(report.Checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(report.Checks))
	}
}

func TestHealthChecker_Check_WithFailure(t *testing.T) {
	h := NewHealthChecker()
	_ = h.RegisterCheck("redis", func(ctx context.Context) error { return nil })
	_ = h.RegisterCheck("mcp", func(ctx context.Context) error { return errors.New("timeout") })

	report := h.Check(context.Background())
	if report.Status != "error" {
		t.Fatalf("expected error, got %s", report.Status)
	}
	var hasFailed bool
	for _, c := range report.Checks {
		if c.Name == "mcp" && c.Status == "error" {
			hasFailed = true
		}
	}
	if !hasFailed {
		t.Fatalf("expected failed mcp check, got %+v", report.Checks)
	}
}

func TestHealthChecker_ServeHTTP(t *testing.T) {
	h := NewHealthChecker()
	_ = h.RegisterCheck("redis", func(ctx context.Context) error { return nil })

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestHealthChecker_ServeHTTP_Unhealthy(t *testing.T) {
	h := NewHealthChecker()
	_ = h.RegisterCheck("db", func(ctx context.Context) error { return errors.New("down") })

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}
