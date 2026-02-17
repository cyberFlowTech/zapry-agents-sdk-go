// Package zapry provides the Zapry platform channel implementation.
// It wraps the Telegram Bot API with Zapry-specific compatibility fixes.
//
// Usage:
//
//	config := zapry.DefaultConfig("YOUR_BOT_TOKEN", "https://openapi.mimo.immo/bot")
//	agent, err := zapry.NewAgent(config)
//	agent.AddCommand("start", func(bot *telegram.AgentAPI, update telegram.Update) {
//	    bot.Send(telegram.NewMessage(update.Message.Chat.ID, "Hello from Zapry!"))
//	})
//	agent.Run()
package zapry

import (
	"github.com/cyberFlowTech/zapry-agents-sdk-go/channel/telegram"
)

// Agent is the Zapry platform agent. It wraps a Telegram agent with
// automatic Zapry compatibility (data normalization + unsupported param stripping).
type Agent struct {
	*telegram.ZapryAgent
}

// NewAgent creates a Zapry agent from configuration.
// Automatically enables Zapry compatibility mode.
func NewAgent(config *telegram.AgentConfig) (*Agent, error) {
	// Force Zapry platform
	config.Platform = "zapry"

	inner, err := telegram.NewZapryAgent(config)
	if err != nil {
		return nil, err
	}

	// Enable compat mode on the underlying Bot API
	inner.Bot.SetZapryCompat(true)

	return &Agent{ZapryAgent: inner}, nil
}

// DefaultConfig creates a Zapry-specific config with sensible defaults.
func DefaultConfig(botToken, apiBaseURL string) *telegram.AgentConfig {
	return &telegram.AgentConfig{
		Platform:    "zapry",
		BotToken:    botToken,
		APIBaseURL:  apiBaseURL,
		RuntimeMode: "webhook",
		WebhookHost: "0.0.0.0",
		WebhookPort: 8443,
	}
}
