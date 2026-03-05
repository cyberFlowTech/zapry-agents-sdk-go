package zapry

import (
	"errors"
	"strings"
	"testing"
)

func TestNewAgentConfigFromEnv_InvalidWebhookPort(t *testing.T) {
	t.Setenv("TG_PLATFORM", "telegram")
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("WEBAPP_PORT", "not-a-number")

	_, err := NewAgentConfigFromEnv()
	if err == nil {
		t.Fatal("expected invalid WEBAPP_PORT error")
	}
	if !strings.Contains(err.Error(), "invalid WEBAPP_PORT") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewAgentConfigFromEnv_WebhookPortOutOfRange(t *testing.T) {
	t.Setenv("TG_PLATFORM", "telegram")
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("WEBAPP_PORT", "70000")

	_, err := NewAgentConfigFromEnv()
	if err == nil {
		t.Fatal("expected out-of-range WEBAPP_PORT error")
	}
	if !strings.Contains(err.Error(), "must be 1-65535") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewAgentConfigFromEnv_InvalidZapryAPIBaseURL(t *testing.T) {
	t.Setenv("TG_PLATFORM", "zapry")
	t.Setenv("ZAPRY_BOT_TOKEN", "test-token")
	t.Setenv("ZAPRY_API_BASE_URL", "not-url")

	_, err := NewAgentConfigFromEnv()
	if err == nil {
		t.Fatal("expected invalid api base url error")
	}
	if !errors.Is(err, ErrInvalidAPIBaseURL) {
		t.Fatalf("expected ErrInvalidAPIBaseURL, got %v", err)
	}
}

func TestNewAgentConfigFromEnv_WebhookModeRequiresURL(t *testing.T) {
	t.Setenv("TG_PLATFORM", "telegram")
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("RUNTIME_MODE", "webhook")
	t.Setenv("TELEGRAM_WEBHOOK_URL", "")

	_, err := NewAgentConfigFromEnv()
	if err == nil {
		t.Fatal("expected webhook url required error")
	}
	if !errors.Is(err, ErrWebhookURLRequired) {
		t.Fatalf("expected ErrWebhookURLRequired, got %v", err)
	}
}

func TestNewAgentConfigFromEnv_InvalidWebhookURL(t *testing.T) {
	t.Setenv("TG_PLATFORM", "telegram")
	t.Setenv("TELEGRAM_BOT_TOKEN", "test-token")
	t.Setenv("TELEGRAM_WEBHOOK_URL", "://broken")

	_, err := NewAgentConfigFromEnv()
	if err == nil {
		t.Fatal("expected invalid webhook url error")
	}
	if !errors.Is(err, ErrInvalidWebhookURL) {
		t.Fatalf("expected ErrInvalidWebhookURL, got %v", err)
	}
}
