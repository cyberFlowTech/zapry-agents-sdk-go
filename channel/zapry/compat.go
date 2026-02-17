package zapry

// Zapry compatibility is handled by the telegram package's built-in
// NormalizeUpdate and NormalizeSendParams functions, which are automatically
// activated when AgentConfig.Platform == "zapry".
//
// This file documents Zapry-specific limitations for developer reference.

// UnsupportedFeatures lists Telegram Bot API features not available on Zapry.
var UnsupportedFeatures = []string{
	"parse_mode",               // Markdown/HTML formatting not supported
	"explanation_parse_mode",   // Quiz explanation formatting not supported
	"editMessageText",          // Message editing not supported
	"sendVoice",                // Voice messages not supported
	"sendChatAction",           // Typing indicators not supported
}

// KnownDataIssues documents Zapry data format differences from Telegram.
var KnownDataIssues = []string{
	"User.FirstName may be empty",
	"Chat.ID may have 'g_' prefix for groups",
	"Chat.Type may be empty",
	"Private Chat.ID may be bot username string instead of numeric ID",
	"Markdown in messages is not rendered",
}
