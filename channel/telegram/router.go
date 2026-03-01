package telegram

import (
	"log"
	"os"
	"regexp"
	"strings"
)

// HandlerFunc is the function signature for all update handlers.
// It receives the low-level AgentAPI and the incoming Update.
type HandlerFunc func(agent *AgentAPI, update Update)

// callbackRoute pairs a regex pattern with a handler.
type callbackRoute struct {
	pattern *regexp.Regexp
	handler HandlerFunc
}

// messageRoute pairs a filter string with a handler.
type messageRoute struct {
	filter  string      // "private", "group", "all"
	handler HandlerFunc
}

// Router dispatches incoming Updates to registered handlers.
//
// Dispatch priority:
//  1. Command handlers (exact match on command name)
//  2. Callback query handlers (regex match on callback data)
//  3. Message handlers (filter match on chat type)
type Router struct {
	commands  map[string]HandlerFunc
	callbacks []callbackRoute
	messages  []messageRoute
	debug     bool
}

// NewRouter creates an empty Router.
func NewRouter() *Router {
	return &Router{
		commands:  make(map[string]HandlerFunc),
		callbacks: make([]callbackRoute, 0),
		messages:  make([]messageRoute, 0),
	}
}

// AddCommand registers a handler for a bot command (e.g. "start" for /start).
func (r *Router) AddCommand(name string, handler HandlerFunc) {
	r.commands[name] = handler
	if r.debug {
		log.Printf("[Router] Registered command: /%s", name)
	}
}

// AddCallbackQuery registers a handler for callback queries matching the regex pattern.
// Example: AddCallbackQuery("^show_detail$", handler)
func (r *Router) AddCallbackQuery(pattern string, handler HandlerFunc) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		log.Printf("[Router] WARNING: invalid callback pattern %q: %v", pattern, err)
		return
	}
	r.callbacks = append(r.callbacks, callbackRoute{pattern: re, handler: handler})
	if r.debug {
		log.Printf("[Router] Registered callback: %s", pattern)
	}
}

// AddMessage registers a handler for text messages matching the filter.
//
// Supported filters:
//   - "private"  — only private (DM) messages
//   - "group"    — only group and supergroup messages
//   - "all"      — all text messages
func (r *Router) AddMessage(filter string, handler HandlerFunc) {
	r.messages = append(r.messages, messageRoute{filter: strings.ToLower(filter), handler: handler})
	if r.debug {
		log.Printf("[Router] Registered message filter: %s", filter)
	}
}

// Dispatch routes an Update to the appropriate handler.
// Returns true if a handler was found and invoked, false otherwise.
func (r *Router) Dispatch(agent *AgentAPI, update Update) bool {
	trace := r.traceEnabled()

	// 1. Command messages
	if update.Message != nil && update.Message.IsCommand() {
		cmd := update.Message.Command()
		if handler, ok := r.commands[cmd]; ok {
			if trace {
				log.Printf("[RouteTrace] matched command=/%s", cmd)
			}
			handler(agent, update)
			return true
		}
		if trace {
			log.Printf("[RouteTrace] command not matched command=/%s", cmd)
		}
		// Unknown command — fall through to message handlers
	}

	// 2. Callback queries
	if update.CallbackQuery != nil {
		data := update.CallbackQuery.Data
		for _, route := range r.callbacks {
			if route.pattern.MatchString(data) {
				if trace {
					log.Printf("[RouteTrace] matched callback pattern=%s data=%q", route.pattern.String(), summarizeRouteText(data, 120))
				}
				route.handler(agent, update)
				return true
			}
		}
		if trace {
			log.Printf("[RouteTrace] callback not matched data=%q", summarizeRouteText(data, 120))
		}
	}

	// 3. Plain text messages (non-command)
	if update.Message != nil && !update.Message.IsCommand() && update.Message.Text != "" {
		chatType := ""
		if update.Message.Chat != nil {
			chatType = strings.ToLower(update.Message.Chat.Type)
		}

		for _, route := range r.messages {
			if matchMessageFilter(route.filter, chatType) {
				if trace {
					log.Printf("[RouteTrace] matched message filter=%s chat_type=%s text=%q", route.filter, chatType, summarizeRouteText(update.Message.Text, 120))
				}
				route.handler(agent, update)
				return true
			}
		}
		if trace {
			filters := make([]string, 0, len(r.messages))
			for _, route := range r.messages {
				filters = append(filters, route.filter)
			}
			log.Printf("[RouteTrace] message not matched chat_type=%s registered_filters=%v text=%q", chatType, filters, summarizeRouteText(update.Message.Text, 120))
		}
	}

	if trace {
		log.Printf("[RouteTrace] update dropped (no handler)")
	}
	return false
}

// matchMessageFilter checks if a chat type matches the filter.
func matchMessageFilter(filter, chatType string) bool {
	switch filter {
	case "all":
		return true
	case "private":
		return chatType == "private"
	case "group":
		return chatType == "group" || chatType == "supergroup"
	default:
		return false
	}
}

func (r *Router) traceEnabled() bool {
	if r.debug {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ZAPRY_ROUTE_TRACE"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func summarizeRouteText(s string, maxRunes int) string {
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
