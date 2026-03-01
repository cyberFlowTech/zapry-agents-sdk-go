package agentsdk

import "strings"

// ──────────────────────────────────────────────
// Capability Layer (Layer B) — 声明式能力结构
//
// 解决三个平台级问题：
// 1. 可发现：市场、路由、handoff 精准知道"谁会什么"
// 2. 可管控：按 skill/tier 做权限、计费、限流
// 3. 可规模化：非开发者也能创建 Agent（配置驱动）
// ──────────────────────────────────────────────

// AgentCapabilities 是声明式能力层，可序列化，
// 用于 DB 存储、市场展示、路由决策、runtime enforcement。
type AgentCapabilities struct {
	Skills       []SkillSpec     `json:"skills"`
	ToolManifest []ToolSpec      `json:"tools,omitempty"`
	Knowledge    []KnowledgeSpec `json:"knowledge,omitempty"`
}

// SkillSpec 声明一个高级能力。
// 一个 Skill 由 Tools + Knowledge 组成，对外暴露 Tags 用于路由和搜索。
type SkillSpec struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Tools       []string    `json:"tools,omitempty"`       // 引用 ToolSpec.Name
	ToolGrants  []ToolGrant `json:"tool_grants,omitempty"` // per-tool 授权（优先于 Tools）
	Knowledge   []string    `json:"knowledge,omitempty"`   // 引用 KnowledgeSpec.ID
	Tags        []string    `json:"tags,omitempty"`
	Tier        string      `json:"tier,omitempty"` // "free" | "premium"
}

// ToolSpec 是工具的声明式元数据（不含运行时 handler）。
type ToolSpec struct {
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Category      string         `json:"category,omitempty"`       // "divination" | "memory" | "payment" ...
	ParamsSummary []string       `json:"params_summary,omitempty"` // 最小参数摘要，如 ["spread:string(required)"]
	RateLimit     *RateLimitSpec `json:"rate_limit,omitempty"`
}

// ToolGrant 将工具绑定到特定权限域，支持 per-tool 频控和计费归因。
type ToolGrant struct {
	ToolName  string         `json:"tool_name"`
	Tier      string         `json:"tier,omitempty"`       // 覆盖 SkillSpec.Tier
	RateLimit *RateLimitSpec `json:"rate_limit,omitempty"` // per-grant 频控
	Scopes    []string       `json:"scopes,omitempty"`     // 权限域，如 ["read", "write"]
}

// KnowledgeSpec 声明 Agent 可访问的知识源。
type KnowledgeSpec struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // "static" | "rag" | "api"
	Description string `json:"description,omitempty"`
}

// RateLimitSpec 定义频率限制。
type RateLimitSpec struct {
	MaxPerMinute int `json:"max_per_minute,omitempty"`
	MaxPerDay    int `json:"max_per_day,omitempty"`
}

// ──────────────────────────────────────────────
// AgentCapabilities 辅助方法
// ──────────────────────────────────────────────

// AllTags 从所有 Skills 中提取去重后的路由标签。
func (c *AgentCapabilities) AllTags() []string {
	if c == nil {
		return nil
	}
	seen := make(map[string]bool)
	var tags []string
	for _, s := range c.Skills {
		for _, t := range s.Tags {
			lower := strings.ToLower(t)
			if !seen[lower] {
				seen[lower] = true
				tags = append(tags, t)
			}
		}
	}
	return tags
}

// AllToolNames 返回 ToolManifest 中所有工具名。
func (c *AgentCapabilities) AllToolNames() []string {
	if c == nil {
		return nil
	}
	names := make([]string, len(c.ToolManifest))
	for i, t := range c.ToolManifest {
		names[i] = t.Name
	}
	return names
}

// HasTool 检查某工具是否在 ToolManifest 中。
func (c *AgentCapabilities) HasTool(name string) bool {
	if c == nil {
		return false
	}
	for _, t := range c.ToolManifest {
		if t.Name == name {
			return true
		}
	}
	return false
}

// FindSkillByTool 查找某工具所属的第一个 Skill。
// 优先匹配 ToolGrants，其次匹配 Tools 列表。
func (c *AgentCapabilities) FindSkillByTool(toolName string) *SkillSpec {
	if c == nil {
		return nil
	}
	for i, s := range c.Skills {
		for _, g := range s.ToolGrants {
			if g.ToolName == toolName {
				return &c.Skills[i]
			}
		}
		for _, t := range s.Tools {
			if t == toolName {
				return &c.Skills[i]
			}
		}
	}
	return nil
}

// FindToolGrant 查找某工具的 ToolGrant 授权规则和归属 Skill。
// 返回 nil, "" 表示该工具不在任何 Skill 的 ToolGrants 中（可能在 Tools 列表中）。
func (c *AgentCapabilities) FindToolGrant(toolName string) (*ToolGrant, string) {
	if c == nil {
		return nil, ""
	}
	for _, s := range c.Skills {
		for i, g := range s.ToolGrants {
			if g.ToolName == toolName {
				return &s.ToolGrants[i], s.Name
			}
		}
	}
	return nil, ""
}

// ──────────────────────────────────────────────
// ToolCallDecision — 授权检查结果
// ──────────────────────────────────────────────

// ToolCallDecision 是 checkToolGrant 的返回值。
// 不只是 allow/deny，还包含频控信息和计费归因。
type ToolCallDecision struct {
	Allowed    bool
	Grant      *ToolGrant // 命中的授权规则（可能为 nil）
	SkillName  string     // 该工具归属的 Skill（计费归因）
	DenyReason string
}

// CheckToolGrant 检查工具调用授权。
// caps 为 nil 或 ToolManifest 为空时，返回 Allowed=true（向后兼容）。
func CheckToolGrant(caps *AgentCapabilities, toolName string) ToolCallDecision {
	if caps == nil || len(caps.ToolManifest) == 0 {
		return ToolCallDecision{Allowed: true}
	}

	if !caps.HasTool(toolName) {
		return ToolCallDecision{Allowed: false, DenyReason: "tool not in capability manifest: " + toolName}
	}

	// 查找 ToolGrant（精确授权）
	grant, skillName := caps.FindToolGrant(toolName)
	if grant != nil {
		return ToolCallDecision{Allowed: true, Grant: grant, SkillName: skillName}
	}

	// 无 ToolGrant 但在某个 Skill 的 Tools 列表中
	skill := caps.FindSkillByTool(toolName)
	if skill != nil {
		return ToolCallDecision{Allowed: true, SkillName: skill.Name}
	}

	// 在 ToolManifest 中但不在任何 Skill 中，仍然允许
	return ToolCallDecision{Allowed: true}
}

// ──────────────────────────────────────────────
// Routing View — 给 Coordinator LLM 路由用
// ──────────────────────────────────────────────

// RoutingView 是 Coordinator LLM 路由的输入。
// 比 AgentCardPublic 更精简，比 Skills tags 更丰富。
type RoutingView struct {
	AgentID        string           `json:"agent_id"`
	DisplayName    string           `json:"display_name"`
	Description    string           `json:"description"`
	SkillSummaries []SkillRouteInfo `json:"skills"`
}

// SkillRouteInfo 是路由视图中单个 Skill 的摘要。
type SkillRouteInfo struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Tags           []string `json:"tags"`
	Tier           string   `json:"tier,omitempty"`
	ToolCategories []string `json:"tool_categories,omitempty"`
}

// ToRoutingView 从 AgentCardPublic 生成路由视图。
func ToRoutingView(card AgentCardPublic) RoutingView {
	view := RoutingView{
		AgentID:     card.AgentID,
		DisplayName: card.DisplayName,
		Description: card.Description,
	}
	if card.Capabilities == nil {
		return view
	}
	for _, skill := range card.Capabilities.Skills {
		categories := extractToolCategories(skill, card.Capabilities.ToolManifest)
		view.SkillSummaries = append(view.SkillSummaries, SkillRouteInfo{
			Name:           skill.Name,
			Description:    skill.Description,
			Tags:           skill.Tags,
			Tier:           skill.Tier,
			ToolCategories: categories,
		})
	}
	return view
}

// extractToolCategories 从 Skill 引用的 Tools 中提取去重后的 Category 列表。
func extractToolCategories(skill SkillSpec, manifest []ToolSpec) []string {
	toolNames := make(map[string]bool)
	for _, t := range skill.Tools {
		toolNames[t] = true
	}
	for _, g := range skill.ToolGrants {
		toolNames[g.ToolName] = true
	}

	seen := make(map[string]bool)
	var categories []string
	for _, ts := range manifest {
		if toolNames[ts.Name] && ts.Category != "" && !seen[ts.Category] {
			seen[ts.Category] = true
			categories = append(categories, ts.Category)
		}
	}
	return categories
}
