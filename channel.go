package agentsdk

// ──────────────────────────────────────────────
// Channel re-exports — stable public API
// ──────────────────────────────────────────────
//
// This file re-exports the most commonly used types from channel/telegram
// so that external users only need a single import:
//
//	import agentsdk "github.com/cyberFlowTech/zapry-agents-sdk-go"
//
//	bot, _ := agentsdk.NewAgentAPI(token)
//	bot.Send(agentsdk.NewMessage(chatID, "hello"))
//
// For platform-specific or advanced types, import the sub-package directly:
//
//	import "github.com/cyberFlowTech/zapry-agents-sdk-go/channel/telegram"

import "github.com/cyberFlowTech/zapry-agents-sdk-go/channel/telegram"

// ─── Core types ───

// AgentAPI is the low-level Bot API client.
type AgentAPI = telegram.AgentAPI

// ZapryAgent is the high-level agent framework with routing and lifecycle.
type ZapryAgent = telegram.ZapryAgent

// AgentConfig holds bot configuration (token, platform, runtime mode, etc.).
type AgentConfig = telegram.AgentConfig

// Update represents an incoming update from the platform.
type Update = telegram.Update

// Message represents a message in a chat.
type Message = telegram.Message

// User represents a Telegram/Zapry user.
type User = telegram.User

// Chat represents a chat (private, group, supergroup, or channel).
type Chat = telegram.Chat

// ─── Request config types ───

// MessageConfig configures a text message to send.
type MessageConfig = telegram.MessageConfig

// PhotoConfig configures a photo message to send.
type PhotoConfig = telegram.PhotoConfig

// ─── Handler & Middleware ───

// HandlerFunc is the function signature for update handlers.
type HandlerFunc = telegram.HandlerFunc

// MiddlewareFunc is the middleware function signature.
type MiddlewareFunc = telegram.MiddlewareFunc

// NextFunc proceeds to the next middleware or the core handler.
type NextFunc = telegram.NextFunc

// MiddlewareContext is the shared context flowing through the middleware pipeline.
type MiddlewareContext = telegram.MiddlewareContext

// Router dispatches incoming updates to registered handlers.
type Router = telegram.Router

// ─── UI types ───

// CallbackQuery represents an incoming callback query from a callback button.
type CallbackQuery = telegram.CallbackQuery

// InlineKeyboardMarkup represents an inline keyboard.
type InlineKeyboardMarkup = telegram.InlineKeyboardMarkup

// InlineKeyboardButton represents one button of an inline keyboard.
type InlineKeyboardButton = telegram.InlineKeyboardButton

// ReplyKeyboardMarkup represents a custom keyboard with reply options.
type ReplyKeyboardMarkup = telegram.ReplyKeyboardMarkup

// KeyboardButton represents one button of the reply keyboard.
type KeyboardButton = telegram.KeyboardButton

// BotCommand represents a bot command.
type BotCommand = telegram.BotCommand

// ─── Constructors ───

// NewAgentAPI creates a new Bot API client with the default endpoint.
var NewAgentAPI = telegram.NewAgentAPI

// NewAgentAPIWithAPIEndpoint creates a Bot API client with a custom endpoint.
var NewAgentAPIWithAPIEndpoint = telegram.NewAgentAPIWithAPIEndpoint

// NewZapryAgent creates a high-level agent from configuration.
var NewZapryAgent = telegram.NewZapryAgent

// NewAgentConfigFromEnv loads configuration from environment variables.
var NewAgentConfigFromEnv = telegram.NewAgentConfigFromEnv

// NewMessage creates a new text message config.
var NewMessage = telegram.NewMessage

// NewPhoto creates a new photo message config.
var NewPhoto = telegram.NewPhoto

// NewRouter creates an empty router.
var NewRouter = telegram.NewRouter

// NewInlineKeyboardMarkup creates an inline keyboard markup.
var NewInlineKeyboardMarkup = telegram.NewInlineKeyboardMarkup

// NewInlineKeyboardRow creates a row of inline keyboard buttons.
var NewInlineKeyboardRow = telegram.NewInlineKeyboardRow

// NewInlineKeyboardButtonData creates an inline keyboard button with callback data.
var NewInlineKeyboardButtonData = telegram.NewInlineKeyboardButtonData

// NewReplyKeyboard creates a reply keyboard markup.
var NewReplyKeyboard = telegram.NewReplyKeyboard

// NewKeyboardButton creates a plain text keyboard button.
var NewKeyboardButton = telegram.NewKeyboardButton

// NewCallback creates a callback query response.
var NewCallback = telegram.NewCallback

// NewSetMyCommands creates a command to set the bot's commands.
var NewSetMyCommands = telegram.NewSetMyCommands

// NewEditMessageText creates an edit message text config.
var NewEditMessageText = telegram.NewEditMessageText

// NewKeyboardButtonRow creates a row of keyboard buttons.
var NewKeyboardButtonRow = telegram.NewKeyboardButtonRow
