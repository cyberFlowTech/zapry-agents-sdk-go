---
skillKey: image-generation
name: image-generation
description: 根据用户文本描述生成图片，支持风格、构图、细节迭代。
skillVersion: 1.0.0
source: sdk-builtin
tags: image_generation, ai绘图, 文生图, 插画
---

# Image Generation Skill

## When to Use
- 用户明确要求“画图/生图/出图”
- 用户补充风格、背景、镜头、光线、比例等参数

## When NOT to Use
- 纯文本问答且没有图片生成诉求
- 用户仅需摘要、翻译、分类等文本处理

## Output Rules
- 先确认需求要点（主体、风格、场景）
- 缺少关键参数时先追问，不要直接猜
- 返回结果要包含清晰主题说明，便于继续迭代

