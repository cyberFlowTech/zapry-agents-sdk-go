package agentsdk

// ──────────────────────────────────────────────
// Channel re-exports — stable public API
// ──────────────────────────────────────────────
//
// This file re-exports the most commonly used types from channel/zapry
// so that external users only need a single import:
//
//	import agentsdk "github.com/cyberFlowTech/zapry-agents-sdk-go"
//
//	bot, _ := agentsdk.NewAgentAPI(token)
//	bot.Send(agentsdk.NewMessage(chatID, "hello"))
//
// For platform-specific or advanced types, import the sub-package directly:
//
//	import "github.com/cyberFlowTech/zapry-agents-sdk-go/channel/zapry"

import "github.com/cyberFlowTech/zapry-agents-sdk-go/channel/zapry"

// ─── Core types ───

// AgentAPI is the low-level Bot API client.
type AgentAPI = zapry.AgentAPI

// ZapryAgent is the high-level agent framework with routing and lifecycle.
type ZapryAgent = zapry.ZapryAgent

// AgentConfig holds bot configuration (token, platform, runtime mode, etc.).
type AgentConfig = zapry.AgentConfig

// ProfileSource is the sovereign source payload for extended profile sync.
type ProfileSource = zapry.ProfileSource

// ProfileSourceSkill represents one SKILL.md snapshot in ProfileSource.
type ProfileSourceSkill = zapry.ProfileSourceSkill

// DerivedProfile is the flattened profile returned by setMyProfile extension.
type DerivedProfile = zapry.DerivedProfile

// Update represents an incoming update from the platform.
type Update = zapry.Update

// Message represents a message in a chat.
type Message = zapry.Message

// User represents a Telegram/Zapry user.
type User = zapry.User

// Chat represents a chat (private, group, supergroup, or channel).
type Chat = zapry.Chat

// ─── Request config types ───

// MessageConfig configures a text message to send.
type MessageConfig = zapry.MessageConfig

// PhotoConfig configures a photo message to send.
type PhotoConfig = zapry.PhotoConfig

// VideoConfig configures a video message to send.
type VideoConfig = zapry.VideoConfig

// DocumentConfig configures a document/file message to send.
type DocumentConfig = zapry.DocumentConfig

// AudioConfig configures an audio message to send.
type AudioConfig = zapry.AudioConfig

// VoiceConfig configures a voice message to send.
type VoiceConfig = zapry.VoiceConfig

// AnimationConfig configures a GIF animation message to send.
type AnimationConfig = zapry.AnimationConfig

// StickerConfig configures a sticker message to send.
type StickerConfig = zapry.StickerConfig

// VideoNoteConfig configures a video note message to send.
type VideoNoteConfig = zapry.VideoNoteConfig

// MediaGroupConfig configures a media group (album) to send.
type MediaGroupConfig = zapry.MediaGroupConfig

// ─── File data types ───

// RequestFileData represents file data for upload or URL reference.
type RequestFileData = zapry.RequestFileData

// FileURL is a URL to use as a file (no upload needed).
type FileURL = zapry.FileURL

// FileBytes contains in-memory bytes to upload as a file.
type FileBytes = zapry.FileBytes

// FileReader wraps an io.Reader to upload as a file.
type FileReader = zapry.FileReader

// FilePath is a path to a local file to upload.
type FilePath = zapry.FilePath

// FileID references a file already on the server.
type FileID = zapry.FileID

// ─── Media types (received messages) ───

// PhotoSize represents one size of a photo.
type PhotoSize = zapry.PhotoSize

// Video represents a video file.
type Video = zapry.Video

// Document represents a general file (non-photo/video/audio).
type Document = zapry.Document

// Audio represents an audio file.
type Audio = zapry.Audio

// Voice represents a voice note.
type Voice = zapry.Voice

// Animation represents a GIF or video without sound.
type Animation = zapry.Animation

// ─── InputMedia types (for MediaGroup) ───

// InputMediaPhoto represents a photo in a media group.
type InputMediaPhoto = zapry.InputMediaPhoto

// InputMediaVideo represents a video in a media group.
type InputMediaVideo = zapry.InputMediaVideo

// InputMediaDocument represents a document in a media group.
type InputMediaDocument = zapry.InputMediaDocument

// InputMediaAudio represents an audio in a media group.
type InputMediaAudio = zapry.InputMediaAudio

// InputMediaAnimation represents an animation in a media group.
type InputMediaAnimation = zapry.InputMediaAnimation

// ─── Handler & Middleware ───

// HandlerFunc is the function signature for update handlers.
type HandlerFunc = zapry.HandlerFunc

// MiddlewareFunc is the middleware function signature.
type MiddlewareFunc = zapry.MiddlewareFunc

// NextFunc proceeds to the next middleware or the core handler.
type NextFunc = zapry.NextFunc

// MiddlewareContext is the shared context flowing through the middleware pipeline.
type MiddlewareContext = zapry.MiddlewareContext

// Router dispatches incoming updates to registered handlers.
type Router = zapry.Router

// ─── UI types ───

// CallbackQuery represents an incoming callback query from a callback button.
type CallbackQuery = zapry.CallbackQuery

// InlineKeyboardMarkup represents an inline keyboard.
type InlineKeyboardMarkup = zapry.InlineKeyboardMarkup

// InlineKeyboardButton represents one button of an inline keyboard.
type InlineKeyboardButton = zapry.InlineKeyboardButton

// ReplyKeyboardMarkup represents a custom keyboard with reply options.
type ReplyKeyboardMarkup = zapry.ReplyKeyboardMarkup

// KeyboardButton represents one button of the reply keyboard.
type KeyboardButton = zapry.KeyboardButton

// BotCommand represents a bot command.
type BotCommand = zapry.BotCommand

// ─── Constructors ───

// NewAgentAPI creates a new Bot API client with the default endpoint.
var NewAgentAPI = zapry.NewAgentAPI

// NewAgentAPIWithAPIEndpoint creates a Bot API client with a custom endpoint.
var NewAgentAPIWithAPIEndpoint = zapry.NewAgentAPIWithAPIEndpoint

// NewZapryAgent creates a high-level agent from configuration.
var NewZapryAgent = zapry.NewZapryAgent

// NewAgentConfigFromEnv loads configuration from environment variables.
var NewAgentConfigFromEnv = zapry.NewAgentConfigFromEnv

// BuildProfileSourceFromDir builds profileSource from SOUL.md + skills/*/SKILL.md.
var BuildProfileSourceFromDir = zapry.BuildProfileSourceFromDir

// SkillKeysFromProfileSource returns unique skill keys from profileSource.
var SkillKeysFromProfileSource = zapry.SkillKeysFromProfileSource

// BuildRuntimeSystemPromptFromSource builds runtime system prompt from source files.
var BuildRuntimeSystemPromptFromSource = zapry.BuildRuntimeSystemPromptFromSource

// NewMessage creates a new text message config.
var NewMessage = zapry.NewMessage

// NewPhoto creates a new photo message config.
var NewPhoto = zapry.NewPhoto

// NewVideo creates a new video message config.
var NewVideo = zapry.NewVideo

// NewDocument creates a new document/file message config.
var NewDocument = zapry.NewDocument

// NewAudio creates a new audio message config.
var NewAudio = zapry.NewAudio

// NewVoice creates a new voice message config.
var NewVoice = zapry.NewVoice

// NewAnimation creates a new GIF animation message config.
var NewAnimation = zapry.NewAnimation

// NewSticker creates a new sticker message config.
var NewSticker = zapry.NewSticker

// NewVideoNote creates a new video note message config.
var NewVideoNote = zapry.NewVideoNote

// NewMediaGroup creates a new media group (album) config.
var NewMediaGroup = zapry.NewMediaGroup

// NewInputMediaPhoto creates a new photo for a media group.
var NewInputMediaPhoto = zapry.NewInputMediaPhoto

// NewInputMediaVideo creates a new video for a media group.
var NewInputMediaVideo = zapry.NewInputMediaVideo

// NewInputMediaDocument creates a new document for a media group.
var NewInputMediaDocument = zapry.NewInputMediaDocument

// NewInputMediaAudio creates a new audio for a media group.
var NewInputMediaAudio = zapry.NewInputMediaAudio

// NewInputMediaAnimation creates a new animation for a media group.
var NewInputMediaAnimation = zapry.NewInputMediaAnimation

// NewRouter creates an empty router.
var NewRouter = zapry.NewRouter

// NewInlineKeyboardMarkup creates an inline keyboard markup.
var NewInlineKeyboardMarkup = zapry.NewInlineKeyboardMarkup

// NewInlineKeyboardRow creates a row of inline keyboard buttons.
var NewInlineKeyboardRow = zapry.NewInlineKeyboardRow

// NewInlineKeyboardButtonData creates an inline keyboard button with callback data.
var NewInlineKeyboardButtonData = zapry.NewInlineKeyboardButtonData

// NewReplyKeyboard creates a reply keyboard markup.
var NewReplyKeyboard = zapry.NewReplyKeyboard

// NewKeyboardButton creates a plain text keyboard button.
var NewKeyboardButton = zapry.NewKeyboardButton

// NewCallback creates a callback query response.
var NewCallback = zapry.NewCallback

// NewSetMyCommands creates a command to set the bot's commands.
var NewSetMyCommands = zapry.NewSetMyCommands

// NewEditMessageText creates an edit message text config.
var NewEditMessageText = zapry.NewEditMessageText

// NewKeyboardButtonRow creates a row of keyboard buttons.
var NewKeyboardButtonRow = zapry.NewKeyboardButtonRow
