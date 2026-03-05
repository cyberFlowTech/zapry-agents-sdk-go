package agentsdk

import "strings"

// BuiltinSkill 在 AgentBuilder 上添加 SDK 内置技能声明（来自 SDK 根目录 skills/*/SKILL.md）。
// 仅能力层，不自动注册工具。
//
// 设计原则：
// 1) 内置技能主要用于“快速声明能力与路由标签”；
// 2) 工具注册仍由开发者显式声明，避免隐式行为；
// 3) 通过 opts 可覆盖默认 tags/tier。
func (b *AgentBuilder) BuiltinSkill(key string, opts ...SkillOption) *AgentBuilder {
	if b == nil {
		return b
	}
	skill, ok := GetBuiltinSkill(strings.TrimSpace(key))
	if !ok {
		return b
	}

	defaultOpts := []SkillOption{
		SkillTags(skill.Tags...),
		SkillTier("free"),
	}
	defaultOpts = append(defaultOpts, opts...)
	return b.Skill(skill.Name, skill.Description, defaultOpts...)
}

// BuiltinSkills 批量挂载 SDK 内置技能声明。
func (b *AgentBuilder) BuiltinSkills(keys ...string) *AgentBuilder {
	if b == nil {
		return b
	}
	for _, key := range keys {
		b.BuiltinSkill(key)
	}
	return b
}

