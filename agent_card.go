package agentsdk

// AgentCardPublic is the serializable Agent metadata (can register to Zapry platform).
type AgentCardPublic struct {
	AgentID              string   `json:"agent_id"`
	Name                 string   `json:"name"`
	Description          string   `json:"description,omitempty"`
	Skills               []string `json:"skills,omitempty"`
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
