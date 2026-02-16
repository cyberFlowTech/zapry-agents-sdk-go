package agentsdk

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// ZapryAgent is the high-level agent framework that wraps AgentAPI with
// handler registration, lifecycle hooks, and automatic polling/webhook detection.
//
// Usage:
//
//	config, _ := NewAgentConfigFromEnv()
//	agent, _ := NewZapryAgent(config)
//
//	agent.AddCommand("start", func(a *AgentAPI, u Update) {
//	    msg := NewMessage(u.Message.Chat.ID, "Hello!")
//	    a.Send(msg)
//	})
//
//	agent.Run()
type ZapryAgent struct {
	// Config is the agent configuration.
	Config *AgentConfig
	// Bot is the underlying low-level AgentAPI.
	Bot *AgentAPI
	// Router handles command/callback/message dispatch.
	Router *Router

	onPostInit func(*ZapryAgent)
	onShutdown func(*ZapryAgent)
	onError    func(*AgentAPI, Update, error)
}

// NewZapryAgent creates a high-level agent from configuration.
// It initializes the underlying AgentAPI with the correct endpoint.
func NewZapryAgent(config *AgentConfig) (*ZapryAgent, error) {
	var bot *AgentAPI
	var err error

	if config.APIBaseURL != "" {
		// Custom API endpoint (Zapry)
		endpoint := config.APIBaseURL + "%s/%s"
		bot, err = NewAgentAPIWithAPIEndpoint(config.BotToken, endpoint)
	} else {
		// Standard Telegram API
		bot, err = NewAgentAPI(config.BotToken)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	bot.Debug = config.Debug

	router := NewRouter()
	router.debug = config.Debug

	return &ZapryAgent{
		Config: config,
		Bot:    bot,
		Router: router,
	}, nil
}

// --- Handler Registration (delegates to Router) ---

// AddCommand registers a handler for a bot command.
func (zb *ZapryAgent) AddCommand(name string, handler HandlerFunc) {
	zb.Router.AddCommand(name, handler)
}

// AddCallbackQuery registers a handler for callback queries matching the pattern.
func (zb *ZapryAgent) AddCallbackQuery(pattern string, handler HandlerFunc) {
	zb.Router.AddCallbackQuery(pattern, handler)
}

// AddMessage registers a handler for text messages.
// filter: "private", "group", or "all".
func (zb *ZapryAgent) AddMessage(filter string, handler HandlerFunc) {
	zb.Router.AddMessage(filter, handler)
}

// --- Lifecycle Hooks ---

// OnPostInit registers a callback that runs after the bot is initialized
// but before it starts receiving updates.
func (zb *ZapryAgent) OnPostInit(fn func(*ZapryAgent)) {
	zb.onPostInit = fn
}

// OnPostShutdown registers a callback that runs when the bot is shutting down.
func (zb *ZapryAgent) OnPostShutdown(fn func(*ZapryAgent)) {
	zb.onShutdown = fn
}

// OnError registers a global error handler for panics in handler functions.
func (zb *ZapryAgent) OnError(fn func(*AgentAPI, Update, error)) {
	zb.onError = fn
}

// --- Run ---

// Run starts the bot. It automatically selects polling or webhook mode
// based on Config.RuntimeMode, and blocks until interrupted.
func (zb *ZapryAgent) Run() {
	log.Printf("[ZapryAgent] %s", zb.Config.Summary())

	// Post-init hook
	if zb.onPostInit != nil {
		zb.onPostInit(zb)
	}

	// Graceful shutdown channel
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if zb.Config.RuntimeMode == "webhook" {
		go zb.runWebhook()
	} else {
		go zb.runPolling()
	}

	log.Printf("[ZapryAgent] Bot is running (mode: %s). Press Ctrl+C to stop.", zb.Config.RuntimeMode)

	// Block until signal
	<-sigChan
	log.Println("[ZapryAgent] Shutting down...")

	if zb.Config.RuntimeMode == "polling" {
		zb.Bot.StopReceivingUpdates()
	}

	// Shutdown hook
	if zb.onShutdown != nil {
		zb.onShutdown(zb)
	}

	log.Println("[ZapryAgent] Goodbye!")
}

// runPolling starts long-polling for updates.
func (zb *ZapryAgent) runPolling() {
	u := NewUpdate(0)
	u.Timeout = 60
	updates := zb.Bot.GetUpdatesChan(u)

	log.Println("[ZapryAgent] Polling for updates...")

	for update := range updates {
		go zb.handleUpdate(update)
	}
}

// runWebhook starts a webhook HTTP server.
func (zb *ZapryAgent) runWebhook() {
	webhookFullURL := zb.Config.WebhookURL
	if zb.Config.WebhookPath != "" {
		webhookFullURL = webhookFullURL + "/" + zb.Config.WebhookPath
	}

	wh, err := NewWebhook(webhookFullURL)
	if err != nil {
		log.Fatalf("[ZapryAgent] Failed to create webhook: %v", err)
	}

	_, err = zb.Bot.Request(wh)
	if err != nil {
		log.Fatalf("[ZapryAgent] Failed to set webhook: %v", err)
	}

	listenPath := "/" + zb.Bot.Token
	if zb.Config.WebhookPath != "" {
		listenPath = "/" + zb.Config.WebhookPath
	}

	updates := zb.Bot.ListenForWebhook(listenPath)

	listenAddr := fmt.Sprintf("%s:%d", zb.Config.WebhookHost, zb.Config.WebhookPort)
	log.Printf("[ZapryAgent] Webhook listening on %s (path: %s)", listenAddr, listenPath)

	go func() {
		if err := http.ListenAndServe(listenAddr, nil); err != nil {
			log.Fatalf("[ZapryAgent] Webhook server error: %v", err)
		}
	}()

	for update := range updates {
		go zb.handleUpdate(update)
	}
}

// handleUpdate processes a single update with panic recovery.
func (zb *ZapryAgent) handleUpdate(update Update) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in handler: %v", r)
			log.Printf("[ZapryAgent] %v", err)
			if zb.onError != nil {
				zb.onError(zb.Bot, update, err)
			}
		}
	}()

	// Normalize Zapry data if needed
	if zb.Config.IsZapry() {
		NormalizeUpdate(&update)
	}

	// Dispatch to registered handlers
	zb.Router.Dispatch(zb.Bot, update)
}
