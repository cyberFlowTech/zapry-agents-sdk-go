package imbotapi

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// ZapryBot is the high-level bot framework that wraps BotAPI with
// handler registration, lifecycle hooks, and automatic polling/webhook detection.
//
// Usage:
//
//	config, _ := NewBotConfigFromEnv()
//	bot, _ := NewZapryBot(config)
//
//	bot.AddCommand("start", func(b *BotAPI, u Update) {
//	    msg := NewMessage(u.Message.Chat.ID, "Hello!")
//	    b.Send(msg)
//	})
//
//	bot.Run()
type ZapryBot struct {
	// Config is the bot configuration.
	Config *BotConfig
	// Bot is the underlying low-level BotAPI.
	Bot *BotAPI
	// Router handles command/callback/message dispatch.
	Router *Router

	onPostInit func(*ZapryBot)
	onShutdown func(*ZapryBot)
	onError    func(*BotAPI, Update, error)
}

// NewZapryBot creates a high-level bot from configuration.
// It initializes the underlying BotAPI with the correct endpoint.
func NewZapryBot(config *BotConfig) (*ZapryBot, error) {
	var bot *BotAPI
	var err error

	if config.APIBaseURL != "" {
		// Custom API endpoint (Zapry)
		endpoint := config.APIBaseURL + "%s/%s"
		bot, err = NewBotAPIWithAPIEndpoint(config.BotToken, endpoint)
	} else {
		// Standard Telegram API
		bot, err = NewBotAPI(config.BotToken)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	bot.Debug = config.Debug

	router := NewRouter()
	router.debug = config.Debug

	return &ZapryBot{
		Config: config,
		Bot:    bot,
		Router: router,
	}, nil
}

// --- Handler Registration (delegates to Router) ---

// AddCommand registers a handler for a bot command.
func (zb *ZapryBot) AddCommand(name string, handler HandlerFunc) {
	zb.Router.AddCommand(name, handler)
}

// AddCallbackQuery registers a handler for callback queries matching the pattern.
func (zb *ZapryBot) AddCallbackQuery(pattern string, handler HandlerFunc) {
	zb.Router.AddCallbackQuery(pattern, handler)
}

// AddMessage registers a handler for text messages.
// filter: "private", "group", or "all".
func (zb *ZapryBot) AddMessage(filter string, handler HandlerFunc) {
	zb.Router.AddMessage(filter, handler)
}

// --- Lifecycle Hooks ---

// OnPostInit registers a callback that runs after the bot is initialized
// but before it starts receiving updates.
func (zb *ZapryBot) OnPostInit(fn func(*ZapryBot)) {
	zb.onPostInit = fn
}

// OnPostShutdown registers a callback that runs when the bot is shutting down.
func (zb *ZapryBot) OnPostShutdown(fn func(*ZapryBot)) {
	zb.onShutdown = fn
}

// OnError registers a global error handler for panics in handler functions.
func (zb *ZapryBot) OnError(fn func(*BotAPI, Update, error)) {
	zb.onError = fn
}

// --- Run ---

// Run starts the bot. It automatically selects polling or webhook mode
// based on Config.RuntimeMode, and blocks until interrupted.
func (zb *ZapryBot) Run() {
	log.Printf("[ZapryBot] %s", zb.Config.Summary())

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

	log.Printf("[ZapryBot] Bot is running (mode: %s). Press Ctrl+C to stop.", zb.Config.RuntimeMode)

	// Block until signal
	<-sigChan
	log.Println("[ZapryBot] Shutting down...")

	if zb.Config.RuntimeMode == "polling" {
		zb.Bot.StopReceivingUpdates()
	}

	// Shutdown hook
	if zb.onShutdown != nil {
		zb.onShutdown(zb)
	}

	log.Println("[ZapryBot] Goodbye!")
}

// runPolling starts long-polling for updates.
func (zb *ZapryBot) runPolling() {
	u := NewUpdate(0)
	u.Timeout = 60
	updates := zb.Bot.GetUpdatesChan(u)

	log.Println("[ZapryBot] Polling for updates...")

	for update := range updates {
		go zb.handleUpdate(update)
	}
}

// runWebhook starts a webhook HTTP server.
func (zb *ZapryBot) runWebhook() {
	webhookFullURL := zb.Config.WebhookURL
	if zb.Config.WebhookPath != "" {
		webhookFullURL = webhookFullURL + "/" + zb.Config.WebhookPath
	}

	wh, err := NewWebhook(webhookFullURL)
	if err != nil {
		log.Fatalf("[ZapryBot] Failed to create webhook: %v", err)
	}

	_, err = zb.Bot.Request(wh)
	if err != nil {
		log.Fatalf("[ZapryBot] Failed to set webhook: %v", err)
	}

	listenPath := "/" + zb.Bot.Token
	if zb.Config.WebhookPath != "" {
		listenPath = "/" + zb.Config.WebhookPath
	}

	updates := zb.Bot.ListenForWebhook(listenPath)

	listenAddr := fmt.Sprintf("%s:%d", zb.Config.WebhookHost, zb.Config.WebhookPort)
	log.Printf("[ZapryBot] Webhook listening on %s (path: %s)", listenAddr, listenPath)

	go func() {
		if err := http.ListenAndServe(listenAddr, nil); err != nil {
			log.Fatalf("[ZapryBot] Webhook server error: %v", err)
		}
	}()

	for update := range updates {
		go zb.handleUpdate(update)
	}
}

// handleUpdate processes a single update with panic recovery.
func (zb *ZapryBot) handleUpdate(update Update) {
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in handler: %v", r)
			log.Printf("[ZapryBot] %v", err)
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
