package zapry

import (
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
