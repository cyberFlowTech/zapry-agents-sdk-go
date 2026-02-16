package agentsdk

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// HandoffMessageData is the unified cross-agent message format.
type HandoffMessageData struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Name    string `json:"name,omitempty"`
}

// HandoffErrorData is a structured error.
type HandoffErrorData struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func (e *HandoffErrorData) Error() string { return e.Code + ": " + e.Message }

// HandoffContextData carries context across agents.
type HandoffContextData struct {
	Messages        []HandoffMessageData  `json:"messages,omitempty"`
	MemorySummary   string                `json:"memory_summary,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	TokenBudget     int                   `json:"token_budget,omitempty"`
	RedactionReport []string              `json:"redaction_report,omitempty"`
	Locale          string                `json:"locale,omitempty"`
}

// HandoffRequestData is the handoff request contract.
type HandoffRequestData struct {
	FromAgent      string             `json:"from_agent"`
	ToAgent        string             `json:"to_agent"`
	Reason         string             `json:"reason"`
	RequestedMode  string             `json:"requested_mode,omitempty"`
	RequestID      string             `json:"request_id"`
	TraceID        string             `json:"trace_id,omitempty"`
	DeadlineMs     int                `json:"deadline_ms"`
	HopCount       int                `json:"hop_count"`
	VisitedAgents  []string           `json:"visited_agents,omitempty"`
	CallerOwnerID  string             `json:"caller_owner_id,omitempty"`
	CallerOrgID    string             `json:"caller_org_id,omitempty"`
	Context        HandoffContextData `json:"context"`
	OrigToolCallID string             `json:"orig_tool_call_id,omitempty"`
}

// NewHandoffRequest creates a request with auto-generated request_id.
func NewHandoffRequest(from, to, reason string) *HandoffRequestData {
	return &HandoffRequestData{
		FromAgent:  from,
		ToAgent:    to,
		Reason:     reason,
		RequestID:  generateID(),
		DeadlineMs: 30000,
	}
}

// HandoffResultData is the handoff result contract.
type HandoffResultData struct {
	Output       string                 `json:"output"`
	AgentID      string                 `json:"agent_id"`
	ShouldReturn bool                   `json:"should_return"`
	Status       string                 `json:"status"`
	Error        *HandoffErrorData      `json:"error,omitempty"`
	Usage        map[string]interface{} `json:"usage,omitempty"`
	DurationMs   float64                `json:"duration_ms"`
	RequestID    string                 `json:"request_id"`
	CacheHit     bool                   `json:"cache_hit"`
}

// ToReturnMessage generates the standardized tool result message.
func (r *HandoffResultData) ToReturnMessage(toolCallID string) map[string]interface{} {
	content, _ := json.Marshal(map[string]interface{}{
		"agent_id":   r.AgentID,
		"status":     r.Status,
		"output":     r.Output,
		"usage":      r.Usage,
		"request_id": r.RequestID,
		"cache_hit":  r.CacheHit,
	})
	return map[string]interface{}{
		"role":         "tool",
		"tool_call_id": toolCallID,
		"name":         "handoff_result",
		"content":      string(content),
	}
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ErrorResult creates a HandoffResultData with an error.
func handoffErrorResult(req *HandoffRequestData, code, message string) *HandoffResultData {
	retryable := code == "TIMEOUT" || code == "MODEL_ERROR"
	return &HandoffResultData{
		AgentID:   req.ToAgent,
		Status:    "error",
		Error:     &HandoffErrorData{Code: code, Message: message, Retryable: retryable},
		RequestID: req.RequestID,
	}
}

// Placeholder to satisfy fmt import
var _ = fmt.Sprintf
