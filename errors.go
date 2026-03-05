package agentsdk

import "errors"

var (
	// Tool execution related errors.
	ErrToolNotFound             = errors.New("agentsdk: tool not found")
	ErrToolNoHandler            = errors.New("agentsdk: tool handler is nil")
	ErrToolMissingRequiredArg   = errors.New("agentsdk: tool missing required argument")
	ErrToolTimeout              = errors.New("agentsdk: tool execution timeout")
	ErrToolCancelled            = errors.New("agentsdk: tool execution canceled")
	ErrLLMFunctionNotConfigured = errors.New("agentsdk: llm function is nil")

	// Auto conversation lifecycle errors.
	ErrAutoConversationShuttingDown = errors.New("agentsdk: auto conversation runtime is shutting down")
)
