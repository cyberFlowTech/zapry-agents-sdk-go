package agentsdk

import (
	"log"
	"strings"
)

// NormalizeUpdate fixes Zapry-specific data format issues in an Update.
//
// Known Zapry issues handled:
//   - User.FirstName may be empty → fallback to UserName
//   - Chat.ID may have "g_" prefix for groups → strip it
//   - Chat.Type may be empty → infer from ID format
//   - Private chat.ID may be bot username string → use From.ID instead
//
// This is called automatically by ZapryAgent.handleUpdate when platform is "zapry".
func NormalizeUpdate(update *Update) {
	if update == nil {
		return
	}

	// Normalize Message
	if update.Message != nil {
		normalizeMessage(update.Message)
	}

	// Normalize CallbackQuery
	if update.CallbackQuery != nil {
		if update.CallbackQuery.From != nil {
			normalizeUser(update.CallbackQuery.From)
		}
		if update.CallbackQuery.Message != nil {
			normalizeMessage(update.CallbackQuery.Message)
		}
	}
}

// normalizeMessage fixes a Message and its nested User/Chat objects.
func normalizeMessage(msg *Message) {
	if msg == nil {
		return
	}

	// Fix From (User)
	if msg.From != nil {
		normalizeUser(msg.From)
	}

	// Fix Chat
	if msg.Chat != nil {
		normalizeChat(msg.Chat, msg.From)
	}
}

// normalizeUser fixes Zapry User data issues.
//
// Zapry issues:
//   - FirstName may be empty string
//   - IsBot may not be set
func normalizeUser(user *User) {
	if user == nil {
		return
	}

	// Fix empty FirstName: fallback to UserName → LastName → ID
	if user.FirstName == "" {
		if user.UserName != "" {
			user.FirstName = user.UserName
		} else if user.LastName != "" {
			user.FirstName = user.LastName
		} else if user.ID != "" {
			user.FirstName = user.ID
		}
		if user.FirstName != "" {
			log.Printf("[Compat] Fixed empty FirstName → %s (user %s)", user.FirstName, user.ID)
		}
	}
}

// normalizeChat fixes Zapry Chat data issues.
//
// Zapry issues:
//   - Group chat ID has "g_" prefix (e.g. "g_117686311051260010")
//   - Private chat ID may be bot username string
//   - Type field may be empty
func normalizeChat(chat *Chat, from *User) {
	if chat == nil {
		return
	}

	// Fix "g_" prefix on group chat IDs
	if strings.HasPrefix(chat.ID, "g_") {
		rawID := chat.ID[2:]
		log.Printf("[Compat] Fixed group Chat.ID: %s → %s", chat.ID, rawID)
		chat.ID = rawID
		// Ensure type is group
		if chat.Type == "" || chat.Type == "private" {
			chat.Type = "group"
		}
	}

	// Fix non-numeric private chat ID (Zapry may return bot username)
	if chat.ID != "" && !isNumericID(chat.ID) && !strings.HasPrefix(chat.ID, "-") {
		// This is likely a bot username, replace with from.ID
		if from != nil && from.ID != "" {
			log.Printf("[Compat] Fixed private Chat.ID: %s → %s", chat.ID, from.ID)
			chat.ID = from.ID
		}
		if chat.Type == "" {
			chat.Type = "private"
		}
	}

	// Fix empty Type
	if chat.Type == "" {
		chat.Type = "private"
		log.Printf("[Compat] Fixed empty Chat.Type → private")
	}
}

// ──────────────────────────────────────────────
// Outgoing request normalization (send-side compat)
// ──────────────────────────────────────────────

// zapryUnsupportedParams lists request parameters that the Zapry platform
// does not support. These are automatically removed when zapryCompat is enabled.
var zapryUnsupportedParams = []string{
	"parse_mode",
	"explanation_parse_mode",
}

// NormalizeSendParams removes parameters that are not supported by the Zapry platform.
//
// Known Zapry limitations handled:
//   - parse_mode is not supported (Markdown/HTML formatting ignored)
//   - explanation_parse_mode is not supported
//
// This is called automatically by AgentAPI.MakeRequest and AgentAPI.UploadFiles
// when zapryCompat is enabled (i.e. platform is "zapry").
func NormalizeSendParams(params Params) {
	if params == nil {
		return
	}
	for _, key := range zapryUnsupportedParams {
		if _, exists := params[key]; exists {
			delete(params, key)
			log.Printf("[Compat] Stripped unsupported param: %s", key)
		}
	}
}

// isNumericID checks if a string looks like a numeric ID.
func isNumericID(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
