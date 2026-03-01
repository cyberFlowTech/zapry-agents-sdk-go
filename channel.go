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

// AgentProfileConfig describes agent skills/persona for Coordinator routing.
// Deprecated: Use ZapryAgent.SetSkills() and SetPersona() instead.
type AgentProfileConfig = telegram.AgentProfile

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

// VideoConfig configures a video message to send.
type VideoConfig = telegram.VideoConfig

// DocumentConfig configures a document/file message to send.
type DocumentConfig = telegram.DocumentConfig

// AudioConfig configures an audio message to send.
type AudioConfig = telegram.AudioConfig

// VoiceConfig configures a voice message to send.
type VoiceConfig = telegram.VoiceConfig

// AnimationConfig configures a GIF animation message to send.
type AnimationConfig = telegram.AnimationConfig

// StickerConfig configures a sticker message to send.
type StickerConfig = telegram.StickerConfig

// VideoNoteConfig configures a video note message to send.
type VideoNoteConfig = telegram.VideoNoteConfig

// MediaGroupConfig configures a media group (album) to send.
type MediaGroupConfig = telegram.MediaGroupConfig

// ─── File data types ───

// RequestFileData represents file data for upload or URL reference.
type RequestFileData = telegram.RequestFileData

// FileURL is a URL to use as a file (no upload needed).
type FileURL = telegram.FileURL

// FileBytes contains in-memory bytes to upload as a file.
type FileBytes = telegram.FileBytes

// FileReader wraps an io.Reader to upload as a file.
type FileReader = telegram.FileReader

// FilePath is a path to a local file to upload.
type FilePath = telegram.FilePath

// FileID references a file already on the server.
type FileID = telegram.FileID

// ─── Media types (received messages) ───

// PhotoSize represents one size of a photo.
type PhotoSize = telegram.PhotoSize

// Video represents a video file.
type Video = telegram.Video

// Document represents a general file (non-photo/video/audio).
type Document = telegram.Document

// Audio represents an audio file.
type Audio = telegram.Audio

// Voice represents a voice note.
type Voice = telegram.Voice

// Animation represents a GIF or video without sound.
type Animation = telegram.Animation

// ─── InputMedia types (for MediaGroup) ───

// InputMediaPhoto represents a photo in a media group.
type InputMediaPhoto = telegram.InputMediaPhoto

// InputMediaVideo represents a video in a media group.
type InputMediaVideo = telegram.InputMediaVideo

// InputMediaDocument represents a document in a media group.
type InputMediaDocument = telegram.InputMediaDocument

// InputMediaAudio represents an audio in a media group.
type InputMediaAudio = telegram.InputMediaAudio

// InputMediaAnimation represents an animation in a media group.
type InputMediaAnimation = telegram.InputMediaAnimation

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

// NewVideo creates a new video message config.
var NewVideo = telegram.NewVideo

// NewDocument creates a new document/file message config.
var NewDocument = telegram.NewDocument

// NewAudio creates a new audio message config.
var NewAudio = telegram.NewAudio

// NewVoice creates a new voice message config.
var NewVoice = telegram.NewVoice

// NewAnimation creates a new GIF animation message config.
var NewAnimation = telegram.NewAnimation

// NewSticker creates a new sticker message config.
var NewSticker = telegram.NewSticker

// NewVideoNote creates a new video note message config.
var NewVideoNote = telegram.NewVideoNote

// NewMediaGroup creates a new media group (album) config.
var NewMediaGroup = telegram.NewMediaGroup

// NewInputMediaPhoto creates a new photo for a media group.
var NewInputMediaPhoto = telegram.NewInputMediaPhoto

// NewInputMediaVideo creates a new video for a media group.
var NewInputMediaVideo = telegram.NewInputMediaVideo

// NewInputMediaDocument creates a new document for a media group.
var NewInputMediaDocument = telegram.NewInputMediaDocument

// NewInputMediaAudio creates a new audio for a media group.
var NewInputMediaAudio = telegram.NewInputMediaAudio

// NewInputMediaAnimation creates a new animation for a media group.
var NewInputMediaAnimation = telegram.NewInputMediaAnimation

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
