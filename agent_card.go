package agentsdk

// AgentCardPublic is the serializable Agent metadata (can register to Zapry platform).
type AgentCardPublic struct {
	AgentID              string   `json:"agent_id"`
	Name                 string   `json:"name"`
	Description          string   `json:"description,omitempty"`
	DisplayName          string   `json:"display_name,omitempty"`   // shown in group chat UI
	Skills               []string `json:"skills,omitempty"`
	Talkativeness        float64  `json:"talkativeness,omitempty"`  // 0.0-1.0, group chat speaking probability
	OwnerID              string   `json:"owner_id,omitempty"`
	OrgID                string   `json:"org_id,omitempty"`
	Visibility           string   `json:"visibility,omitempty"`     // private | org | public
	AllowedCallerAgents  []string `json:"allowed_caller_agents,omitempty"`
	AllowedCallerOwners  []string `json:"allowed_caller_owners,omitempty"`
	RequiredScopes       []string `json:"required_scopes,omitempty"`
	SafetyLevel          string   `json:"safety_level,omitempty"`   // low | medium | high
	HandoffPolicyStr     string   `json:"handoff_policy,omitempty"` // auto | coordinator_only | deny
}

// AgentRuntimeConfig is the local runtime binding (not serializable).
type AgentRuntimeConfig struct {
	Card         AgentCardPublic
	LLMFn        LLMFunc
	ToolReg      *ToolRegistry
	SystemPrompt string
	MaxTurns     int
	InputFilter  func(*HandoffContextData) *HandoffContextData
	Guardrails   *GuardrailManager
	Tracer       *AgentTracer
}
