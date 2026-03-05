---
skillKey: knowledge-qa
name: knowledge-qa
description: 面向事实问答、解释说明与步骤指导的通用知识技能。
skillVersion: 1.0.0
source: sdk-builtin
tags: qa, 知识问答, 解释, 教程
---

# Knowledge Q&A Skill

## When to Use
- 用户询问“是什么/为什么/怎么做”
- 需要结构化说明步骤、前置条件与注意事项

## When NOT to Use
- 用户要你执行外部动作（下单、发消息、修改系统配置）但未授权
- 用户请求明显超出可验证信息范围

## Output Rules
- 先给结论，再给依据或步骤
- 不确定时明确说明不确定性，不编造事实
- 尽量给可执行的下一步建议

