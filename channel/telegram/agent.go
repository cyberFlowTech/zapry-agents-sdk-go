package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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

	onPostInit  func(*ZapryAgent)
	onShutdown  func(*ZapryAgent)
	onError     func(*AgentAPI, Update, error)
	pipeline    *MiddlewarePipeline
	pollingLock *pollingInstanceLock
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

	// Enable Zapry send-side compatibility (auto-strip unsupported params)
	if config.IsZapry() {
		bot.SetZapryCompat(true)
	}

	router := NewRouter()
	router.debug = config.Debug

	return &ZapryAgent{
		Config:   config,
		Bot:      bot,
		Router:   router,
		pipeline: NewMiddlewarePipeline(),
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

// --- Middleware ---

// Use registers a global middleware (onion model).
// Middlewares execute in registration order, wrapping the handler dispatch.
// Each middleware receives (ctx, next) and must call next() to proceed.
//
// Example:
//
//	agent.Use(func(ctx *agentsdk.MiddlewareContext, next agentsdk.NextFunc) {
//	    log.Println("before handler")
//	    next()
//	    log.Println("after handler")
//	})
func (zb *ZapryAgent) Use(mw MiddlewareFunc) {
	zb.pipeline.Use(mw)
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

// SetSkills declares the agent's skills for Coordinator routing.
// These are registered with the platform on Run().
// The server auto-generates description/experience/tags via AI.
func (zb *ZapryAgent) SetSkills(skills []string) {
	zb.Config.Skills = skills
}

// SetPersona declares the agent's character/personality for Coordinator routing.
// This is registered with the platform on Run().
func (zb *ZapryAgent) SetPersona(persona string) {
	zb.Config.Persona = persona
}

// SetProfile explicitly sets the full agent profile (deprecated).
// Prefer SetSkills() + SetPersona() instead; the server auto-generates the rest.
func (zb *ZapryAgent) SetProfile(p *AgentProfile) {
	zb.Config.Profile = p
}

// registerProfile sends skills + persona to the platform via POST /setMyProfile.
// If the legacy Profile field is set, it is sent as-is for backward compatibility.
func (zb *ZapryAgent) registerProfile() {
	if zb.Config.APIBaseURL == "" {
		return
	}

	var payload interface{}
	if zb.Config.Profile != nil {
		payload = zb.Config.Profile
	} else if len(zb.Config.Skills) > 0 || strings.TrimSpace(zb.Config.Persona) != "" {
		payload = map[string]interface{}{
			"skills":  zb.Config.Skills,
			"persona": strings.TrimSpace(zb.Config.Persona),
		}
	} else {
		return
	}

	url := fmt.Sprintf("%s%s/setMyProfile", zb.Config.APIBaseURL, zb.Config.BotToken)
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[ZapryAgent] Failed to marshal profile: %v", err)
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[ZapryAgent] Failed to create profile request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := zb.Bot.Client.Do(req)
	if err != nil {
		log.Printf("[ZapryAgent] Failed to register profile: %v", err)
		return
	}
	defer resp.Body.Close()

	skills := zb.Config.Skills
	if zb.Config.Profile != nil {
		skills = zb.Config.Profile.Skills
	}
	if resp.StatusCode == http.StatusOK {
		log.Printf("[ZapryAgent] Profile registered (skills=%v)", skills)
	} else {
		log.Printf("[ZapryAgent] Profile registration returned status %d", resp.StatusCode)
	}
}

// --- Run ---

// Run starts the bot. It automatically selects polling or webhook mode
// based on Config.RuntimeMode, and blocks until interrupted.
func (zb *ZapryAgent) Run() {
	log.Printf("[ZapryAgent] %s", zb.Config.Summary())

	if zb.Config.RuntimeMode == "polling" {
		lock, err := acquirePollingInstanceLock(zb.Config.BotToken)
		if err != nil {
			log.Fatalf("[ZapryAgent] Failed to start polling: %v", err)
		}
		zb.pollingLock = lock
		log.Printf("[ZapryAgent] Polling singleton lock acquired")
	}

	// Post-init hook
	if zb.onPostInit != nil {
		zb.onPostInit(zb)
	}

	// Register profile with the platform (non-blocking, best-effort)
	if zb.Config.IsZapry() {
		zb.registerProfile()
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
		if zb.pollingLock != nil {
			if err := zb.pollingLock.Release(); err != nil {
				log.Printf("[ZapryAgent] Warning: failed to release polling lock: %v", err)
			}
			zb.pollingLock = nil
		}
	}

	// webhook 模式关闭时清理注册，避免 im-provider 继续往已停止的服务推送
	if zb.Config.RuntimeMode == "webhook" {
		if _, err := zb.Bot.Request(DeleteWebhookConfig{}); err != nil {
			log.Printf("[ZapryAgent] Warning: failed to delete webhook on shutdown: %v", err)
		}
	}

	// Shutdown hook
	if zb.onShutdown != nil {
		zb.onShutdown(zb)
	}

	log.Println("[ZapryAgent] Goodbye!")
}

// runPolling starts long-polling for updates.
func (zb *ZapryAgent) runPolling() {
	// 清除可能残留的旧 webhook 注册，否则 im-provider 会继续往已失效的 webhook 地址推送，
	// 导致消息不会写入 Redis，polling 的 getUpdates 永远读不到数据
	if _, err := zb.Bot.Request(DeleteWebhookConfig{}); err != nil {
		log.Printf("[ZapryAgent] Warning: failed to delete webhook: %v", err)
	} else {
		log.Println("[ZapryAgent] Cleared existing webhook for polling mode")
	}

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
	// Determine the webhook path: use explicit WebhookPath, or default to bot token
	webhookPath := zb.Config.WebhookPath
	if webhookPath == "" {
		webhookPath = zb.Bot.Token
	}

	// Build the full URL for registering with the platform
	webhookFullURL := zb.Config.WebhookURL + "/" + webhookPath

	wh, err := NewWebhook(webhookFullURL)
	if err != nil {
		log.Fatalf("[ZapryAgent] Failed to create webhook: %v", err)
	}

	_, err = zb.Bot.Request(wh)
	if err != nil {
		log.Fatalf("[ZapryAgent] Failed to set webhook: %v", err)
	}

	// Listen on the same path
	listenPath := "/" + webhookPath

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

	trace := zb.routeTraceEnabled()
	if trace {
		log.Printf("[RouteTrace] incoming %s", summarizeUpdateForTrace(update))
	}

	// Normalize Zapry data if needed
	if zb.Config.IsZapry() {
		NormalizeUpdate(&update)
		if trace {
			log.Printf("[RouteTrace] normalized %s", summarizeUpdateForTrace(update))
		}
	}

	handled := false
	middlewareHandled := false

	// Run through middleware pipeline → Router.Dispatch as core
	if zb.pipeline.Len() > 0 {
		ctx := &MiddlewareContext{
			Update: update,
			Agent:  zb.Bot,
			Extra:  make(map[string]interface{}),
		}
		zb.pipeline.Execute(ctx, func() {
			handled = zb.Router.Dispatch(zb.Bot, update)
			ctx.Handled = handled
		})
		middlewareHandled = ctx.Handled
	} else {
		handled = zb.Router.Dispatch(zb.Bot, update)
	}

	if trace {
		log.Printf("[RouteTrace] done handled=%t middleware_handled=%t %s", handled, middlewareHandled, summarizeUpdateForTrace(update))
	}
}

func (zb *ZapryAgent) routeTraceEnabled() bool {
	if zb != nil && zb.Config != nil && zb.Config.Debug {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ZAPRY_ROUTE_TRACE"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func summarizeUpdateForTrace(update Update) string {
	if update.Message != nil {
		chatType := ""
		chatID := ""
		if update.Message.Chat != nil {
			chatType = strings.ToLower(strings.TrimSpace(update.Message.Chat.Type))
			chatID = update.Message.Chat.ID
		}
		fromID := ""
		fromBot := false
		if update.Message.From != nil {
			fromID = update.Message.From.ID
			fromBot = update.Message.From.IsBot
		}
		cmd := ""
		if update.Message.IsCommand() {
			cmd = update.Message.Command()
		}
		return fmt.Sprintf(
			"update_id=%d kind=message chat_id=%s chat_type=%s from=%s from_is_bot=%t is_command=%t command=%q text=%q",
			update.UpdateID,
			chatID,
			chatType,
			fromID,
			fromBot,
			update.Message.IsCommand(),
			cmd,
			summarizeTextForTrace(update.Message.Text, 160),
		)
	}
	if update.CallbackQuery != nil {
		fromID := ""
		if update.CallbackQuery.From != nil {
			fromID = update.CallbackQuery.From.ID
		}
		chatID := ""
		chatType := ""
		if update.CallbackQuery.Message != nil && update.CallbackQuery.Message.Chat != nil {
			chatID = update.CallbackQuery.Message.Chat.ID
			chatType = strings.ToLower(strings.TrimSpace(update.CallbackQuery.Message.Chat.Type))
		}
		return fmt.Sprintf(
			"update_id=%d kind=callback chat_id=%s chat_type=%s from=%s data=%q",
			update.UpdateID,
			chatID,
			chatType,
			fromID,
			summarizeTextForTrace(update.CallbackQuery.Data, 160),
		)
	}
	if update.MyChatMember != nil {
		chatID := update.MyChatMember.Chat.ID
		return fmt.Sprintf("update_id=%d kind=my_chat_member chat_id=%s", update.UpdateID, chatID)
	}
	if update.ChatMember != nil {
		chatID := update.ChatMember.Chat.ID
		return fmt.Sprintf("update_id=%d kind=chat_member chat_id=%s", update.UpdateID, chatID)
	}
	return fmt.Sprintf("update_id=%d kind=other", update.UpdateID)
}

func summarizeTextForTrace(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	normalized := strings.ReplaceAll(s, "\n", "\\n")
	runes := []rune(normalized)
	if len(runes) <= maxRunes {
		return normalized
	}
	return string(runes[:maxRunes]) + "..."
}
