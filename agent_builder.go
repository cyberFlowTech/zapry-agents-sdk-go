package agentsdk

import (
	"fmt"
	"strings"
)

// ──────────────────────────────────────────────
// AgentBuilder — 流式 API 构建 Agent
//
// 一段代码同时完成：
// 1. 能力声明（AgentCapabilities — 可序列化）
// 2. 工具注册（ToolRegistry — 运行时）
// 3. AgentCard 生成（元数据）
// 4. 交叉验证（Skill 引用的 Tool/Knowledge 必须存在）
// ──────────────────────────────────────────────

// AgentBuilder 提供流式 API 构建带 Capabilities 的 Agent。
type AgentBuilder struct {
	id           string
	displayName  string
	description  string
	skills       []SkillSpec
	tools        map[string]*Tool
	toolSpecs    []ToolSpec
	knowledge    []KnowledgeSpec
	llmFn        LLMFunc
	llmFnCtx     LLMFuncWithContext
	systemPrompt string
	maxTurns     int
	guardrails   *GuardrailManager
	tracer       *AgentTracer
	visibility   string
	ownerID      string
	orgID        string
	talkativeness float64
	handoffPolicy string
	safetyLevel   string
}

// NewAgentBuilder 创建一个 AgentBuilder。
func NewAgentBuilder(id, displayName string) *AgentBuilder {
	return &AgentBuilder{
		id:          id,
		displayName: displayName,
		tools:       make(map[string]*Tool),
		maxTurns:    10,
		visibility:  "private",
	}
}

func (b *AgentBuilder) Description(d string) *AgentBuilder {
	b.description = d
	return b
}

// Skill 声明一个高级能力，通过 SkillOption 配置 tools/knowledge/tags/tier。
func (b *AgentBuilder) Skill(name, description string, opts ...SkillOption) *AgentBuilder {
	spec := SkillSpec{Name: name, Description: description}
	for _, opt := range opts {
		opt(&spec)
	}
	b.skills = append(b.skills, spec)
	return b
}

// Tool 注册一个运行时工具，同时添加到声明清单。
func (b *AgentBuilder) Tool(name, description string, handler ToolHandlerFunc, params ...ToolParam) *AgentBuilder {
	b.tools[name] = &Tool{
		Name:        name,
		Description: description,
		Parameters:  params,
		Handler:     handler,
	}
	b.toolSpecs = append(b.toolSpecs, ToolSpec{
		Name:          name,
		Description:   description,
		ParamsSummary: buildParamsSummary(params),
	})
	return b
}

// ToolWithCategory 注册工具并指定类别。
func (b *AgentBuilder) ToolWithCategory(name, description, category string, handler ToolHandlerFunc, params ...ToolParam) *AgentBuilder {
	b.Tool(name, description, handler, params...)
	b.toolSpecs[len(b.toolSpecs)-1].Category = category
	return b
}

// Knowledge 声明一个知识源。
func (b *AgentBuilder) Knowledge(id, name, typ, desc string) *AgentBuilder {
	b.knowledge = append(b.knowledge, KnowledgeSpec{ID: id, Name: name, Type: typ, Description: desc})
	return b
}

func (b *AgentBuilder) LLM(fn LLMFunc) *AgentBuilder               { b.llmFn = fn; return b }
func (b *AgentBuilder) LLMCtx(fn LLMFuncWithContext) *AgentBuilder  { b.llmFnCtx = fn; return b }
func (b *AgentBuilder) SystemPrompt(p string) *AgentBuilder         { b.systemPrompt = p; return b }
func (b *AgentBuilder) MaxTurns(n int) *AgentBuilder                { b.maxTurns = n; return b }
func (b *AgentBuilder) WithGuardrails(g *GuardrailManager) *AgentBuilder { b.guardrails = g; return b }
func (b *AgentBuilder) WithTracer(t *AgentTracer) *AgentBuilder     { b.tracer = t; return b }
func (b *AgentBuilder) Visibility(v string) *AgentBuilder           { b.visibility = v; return b }
func (b *AgentBuilder) OwnerID(id string) *AgentBuilder             { b.ownerID = id; return b }
func (b *AgentBuilder) OrgID(id string) *AgentBuilder               { b.orgID = id; return b }
func (b *AgentBuilder) Talkativeness(t float64) *AgentBuilder       { b.talkativeness = t; return b }
func (b *AgentBuilder) HandoffPolicy(p string) *AgentBuilder        { b.handoffPolicy = p; return b }
func (b *AgentBuilder) SafetyLevel(l string) *AgentBuilder          { b.safetyLevel = l; return b }

// Build 校验并生成 AgentRuntimeConfig。
// Skill 引用的 Tool / Knowledge 必须已注册/声明，否则返回 error。
func (b *AgentBuilder) Build() (*AgentRuntimeConfig, error) {
	if err := b.validate(); err != nil {
		return nil, err
	}

	reg := NewToolRegistry()
	for _, t := range b.tools {
		reg.Register(t)
	}

	caps := &AgentCapabilities{
		Skills:       b.skills,
		ToolManifest: b.toolSpecs,
		Knowledge:    b.knowledge,
	}

	config := &AgentRuntimeConfig{
		Card: AgentCardPublic{
			AgentID:          b.id,
			Name:             b.id,
			DisplayName:      b.displayName,
			Description:      b.description,
			Skills:           caps.AllTags(), // 向后兼容：自动填充旧字段
			Capabilities:     caps,
			Talkativeness:    b.talkativeness,
			OwnerID:          b.ownerID,
			OrgID:            b.orgID,
			Visibility:       b.visibility,
			SafetyLevel:      b.safetyLevel,
			HandoffPolicyStr: b.handoffPolicy,
		},
		LLMFn:       b.llmFn,
		ToolReg:     reg,
		SystemPrompt: b.systemPrompt,
		MaxTurns:     b.maxTurns,
		Guardrails:   b.guardrails,
		Tracer:       b.tracer,
	}

	return config, nil
}

// ──────────────────────────────────────────────
// 校验
// ──────────────────────────────────────────────

func (b *AgentBuilder) validate() error {
	if b.id == "" {
		return fmt.Errorf("agent id is required")
	}
	if b.displayName == "" {
		return fmt.Errorf("agent display_name is required")
	}

	// Skill 名称不允许重复
	skillNames := make(map[string]bool)
	for _, s := range b.skills {
		if s.Name == "" {
			return fmt.Errorf("skill name is required")
		}
		lower := strings.ToLower(s.Name)
		if skillNames[lower] {
			return fmt.Errorf("duplicate skill name: %q", s.Name)
		}
		skillNames[lower] = true
	}

	// Tool 名称不允许重复（已通过 map key 保证，但 toolSpecs 需检查）
	toolNames := make(map[string]bool)
	for _, ts := range b.toolSpecs {
		if toolNames[ts.Name] {
			return fmt.Errorf("duplicate tool name: %q", ts.Name)
		}
		toolNames[ts.Name] = true
	}

	// Knowledge ID 不允许重复
	knowledgeIDs := make(map[string]bool)
	for _, k := range b.knowledge {
		if knowledgeIDs[k.ID] {
			return fmt.Errorf("duplicate knowledge id: %q", k.ID)
		}
		knowledgeIDs[k.ID] = true
	}

	// Skill 引用的 Tool 必须已注册
	for _, skill := range b.skills {
		for _, toolName := range skill.Tools {
			if _, ok := b.tools[toolName]; !ok {
				return fmt.Errorf("skill %q references tool %q which is not registered", skill.Name, toolName)
			}
		}
		for _, grant := range skill.ToolGrants {
			if _, ok := b.tools[grant.ToolName]; !ok {
				return fmt.Errorf("skill %q has tool_grant for %q which is not registered", skill.Name, grant.ToolName)
			}
		}
	}

	// Skill 引用的 Knowledge 必须已声明
	for _, skill := range b.skills {
		for _, kid := range skill.Knowledge {
			if !knowledgeIDs[kid] {
				return fmt.Errorf("skill %q references knowledge %q which is not declared", skill.Name, kid)
			}
		}
	}

	return nil
}

// ──────────────────────────────────────────────
// SkillOption — Skill 配置选项
// ──────────────────────────────────────────────

// SkillOption 配置 SkillSpec 的选项函数。
type SkillOption func(*SkillSpec)

// SkillTools 设置 Skill 引用的工具列表。
func SkillTools(names ...string) SkillOption {
	return func(s *SkillSpec) { s.Tools = names }
}

// SkillToolGrants 设置 Skill 的 per-tool 授权规则。
func SkillToolGrants(grants ...ToolGrant) SkillOption {
	return func(s *SkillSpec) { s.ToolGrants = grants }
}

// SkillKnowledge 设置 Skill 引用的知识源。
func SkillKnowledge(ids ...string) SkillOption {
	return func(s *SkillSpec) { s.Knowledge = ids }
}

// SkillTags 设置 Skill 的路由标签。
func SkillTags(tags ...string) SkillOption {
	return func(s *SkillSpec) { s.Tags = tags }
}

// SkillTier 设置 Skill 的付费层级。
func SkillTier(tier string) SkillOption {
	return func(s *SkillSpec) { s.Tier = tier }
}

// ──────────────────────────────────────────────
// 辅助函数
// ──────────────────────────────────────────────

// buildParamsSummary 从 ToolParam 列表生成参数摘要。
func buildParamsSummary(params []ToolParam) []string {
	if len(params) == 0 {
		return nil
	}
	summary := make([]string, len(params))
	for i, p := range params {
		s := p.Name + ":" + p.Type
		if p.Required {
			s += "(required)"
		}
		summary[i] = s
	}
	return summary
}
