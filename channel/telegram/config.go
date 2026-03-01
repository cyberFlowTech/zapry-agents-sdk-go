package telegram

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// AgentProfile describes the agent's skills, persona and capabilities for
// platform-side routing (e.g. Coordinator LLM-based agent selection).
// Deprecated: Use AgentConfig.Skills and AgentConfig.Persona directly, or
// call ZapryAgent.SetSkills() / SetPersona(). The server auto-generates the
// remaining profile fields (description, experience, tags) via AI.
type AgentProfile struct {
	Description string   `json:"description,omitempty"`
	Skills      []string `json:"skills,omitempty"`
	Persona     string   `json:"persona,omitempty"`
	Experience  []string `json:"experience,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Locale      string   `json:"locale,omitempty"`
}

// AgentConfig holds all configuration needed to create and run a ZapryAgent.
// Use NewAgentConfigFromEnv() to load from environment variables (.env file).
type AgentConfig struct {
	// Platform: "telegram" or "zapry"
	Platform string
	// BotToken for the selected platform
	BotToken string
	// APIBaseURL for custom API endpoint (Zapry uses https://openapi.mimo.immo/bot)
	APIBaseURL string
	// RuntimeMode: "polling" or "webhook"
	RuntimeMode string
	// WebhookURL is the public URL for webhook mode
	WebhookURL string
	// WebhookPath is the URL path suffix for webhook
	WebhookPath string
	// WebhookHost is the listen address (default "0.0.0.0")
	WebhookHost string
	// WebhookPort is the listen port (default 8443)
	WebhookPort int
	// WebhookSecret is the secret token for webhook verification
	WebhookSecret string
	// Debug enables verbose logging
	Debug bool
	// LogFile path for file logging (empty = stdout only)
	LogFile string

	// Skills declares what this agent can do (e.g. ["塔罗占卜", "八字命理"]).
	// Set via code (SetSkills) or env fallback (AGENT_SKILLS, comma-separated).
	// The server auto-generates description/experience/tags from skills + persona.
	Skills []string
	// Persona describes the agent's character/personality.
	// Set via code (SetPersona) or env fallback (AGENT_PERSONA).
	Persona string

	// Profile is the legacy full profile. Deprecated: use Skills + Persona instead.
	// If Profile is set, it takes precedence for backward compatibility.
	Profile *AgentProfile
}

// NewAgentConfigFromEnv loads configuration from environment variables.
// It supports automatic platform detection: set TG_PLATFORM to "zapry"
// or "telegram" and the corresponding token/URL will be selected.
//
// Call loadDotEnv() before this if you want .env file support.
func NewAgentConfigFromEnv() (*AgentConfig, error) {
	// Try to load .env file (ignore error if not found)
	loadDotEnv()

	platform := strings.ToLower(strings.TrimSpace(getEnv("TG_PLATFORM", "telegram")))
	if platform != "telegram" && platform != "zapry" {
		platform = "telegram"
	}

	var botToken, apiBaseURL, webhookURL string

	if platform == "zapry" {
		botToken = getEnv("ZAPRY_BOT_TOKEN", "")
		apiBaseURL = getEnv("ZAPRY_API_BASE_URL", "https://openapi.mimo.immo/bot")
		webhookURL = getEnv("ZAPRY_WEBHOOK_URL", "")
	} else {
		botToken = getEnv("TELEGRAM_BOT_TOKEN", "")
		apiBaseURL = "" // Use default Telegram API
		webhookURL = getEnv("TELEGRAM_WEBHOOK_URL", "")
	}

	if botToken == "" {
		return nil, fmt.Errorf("bot token not configured: set %s in environment",
			map[bool]string{true: "ZAPRY_BOT_TOKEN", false: "TELEGRAM_BOT_TOKEN"}[platform == "zapry"])
	}

	runtimeMode := strings.ToLower(strings.TrimSpace(getEnv("RUNTIME_MODE", "polling")))
	if runtimeMode != "webhook" && runtimeMode != "polling" {
		runtimeMode = "polling"
	}

	webhookPort, _ := strconv.Atoi(getEnv("WEBAPP_PORT", "8443"))

	var skills []string
	if raw := getEnv("AGENT_SKILLS", ""); raw != "" {
		for _, s := range strings.Split(raw, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				skills = append(skills, s)
			}
		}
	}
	persona := getEnv("AGENT_PERSONA", "")

	return &AgentConfig{
		Platform:      platform,
		BotToken:      botToken,
		APIBaseURL:    apiBaseURL,
		RuntimeMode:   runtimeMode,
		WebhookURL:    webhookURL,
		WebhookPath:   getEnv("WEBHOOK_PATH", ""),
		WebhookHost:   getEnv("WEBAPP_HOST", "0.0.0.0"),
		WebhookPort:   webhookPort,
		WebhookSecret: getEnv("WEBHOOK_SECRET_TOKEN", ""),
		Debug:         toBool(getEnv("DEBUG", "false")),
		LogFile:       getEnv("LOG_FILE", ""),
		Skills:        skills,
		Persona:       persona,
	}, nil
}

// Summary returns a human-readable configuration summary with sensitive data masked.
func (c *AgentConfig) Summary() string {
	tokenDisplay := c.BotToken
	if len(tokenDisplay) > 10 {
		tokenDisplay = tokenDisplay[:10] + "..."
	}
	return fmt.Sprintf(
		"Platform: %s | Token: %s | Mode: %s | API: %s | Debug: %v",
		strings.ToUpper(c.Platform),
		tokenDisplay,
		c.RuntimeMode,
		defaultStr(c.APIBaseURL, "Telegram Official"),
		c.Debug,
	)
}

// IsZapry returns true if the platform is Zapry.
func (c *AgentConfig) IsZapry() bool {
	return c.Platform == "zapry"
}

// --- internal helpers ---

func getEnv(key, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return strings.TrimSpace(val)
}

func toBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes" || s == "on"
}

func defaultStr(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// loadDotEnv attempts to load a .env file from the current directory.
// It silently ignores errors (file not found, parse errors).
func loadDotEnv() {
	data, err := os.ReadFile(".env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Don't override existing env vars
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
