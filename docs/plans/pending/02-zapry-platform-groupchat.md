# Zapry 平台侧需求：支持 AI Agent 群聊

## 背景

SDK 已完成多 Agent 群聊协议（GroupChat），包含消息路由、发言策略、共享上下文和成员介绍。SDK 层是纯逻辑，不依赖任何平台特性。

本文档描述 Zapry 平台侧需要配合的改动，使群聊中的 AI Agent 对用户可见、可交互。

---

## 改动 1（必须）：群成员支持 AI Agent 类型

### 当前状态

Zapry 群聊的成员表只有人类用户（user_id, nickname, avatar）。

### 需要改动

群成员表新增 `member_type` 字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `member_type` | string | `"human"` 或 `"agent"` |
| `agent_id` | string | 当 member_type="agent" 时，对应 SDK 的 AgentCardPublic.AgentID |

### API 改动

- `POST /group/{id}/members` — 支持添加 agent 类型成员
- `DELETE /group/{id}/members/{member_id}` — 支持移除 agent 成员
- `GET /group/{id}/members` — 返回结果中包含 member_type 字段

### 前端改动

- 群成员列表中，AI Agent 带标识（如名字后面加 `[AI]` 标签或机器人图标）
- Agent 的头像使用 AgentCardPublic.DisplayName 或默认机器人头像

### 优先级：P0

没有这个改动，用户无法在群聊中看到和管理 AI Agent。

---

## 改动 2（必须）：Webhook 消息携带群聊上下文

### 当前状态

Zapry 通过 Webhook 推送消息给 Bot 进程。当前推送的 Update 结构（兼容 Telegram Bot API）已包含：

- `Message.Chat.ID` — 群聊 ID
- `Message.Chat.Type` — "group" / "supergroup"
- `Message.From` — 发送者信息
- `Message.Text` — 消息内容

### 需要确认/补充的字段

| 字段 | 位置 | 说明 | 当前状态 |
|------|------|------|---------|
| `Message.Entities` | Telegram 标准字段 | 包含 @mention 信息 | 需确认 Zapry 是否支持 |
| `Message.Chat.Type` | Telegram 标准字段 | 需要正确返回 "group" | 需确认 |
| `Message.ReplyToMessage` | Telegram 标准字段 | 回复某条消息时的引用 | 需确认 |

### 关键需求

SDK 的 GroupChatRouter 需要知道用户 @了哪个 Agent。有两种方式：

**方式 A（推荐）**：Zapry 支持 `@agent_name` mention，并在 `Message.Entities` 中返回 `type: "mention"` 实体，像 Telegram 一样。

**方式 B（兜底）**：Bot 进程自己解析消息文本中的 `@xxx`，匹配已注册的 Agent 名字。不需要平台改动，但准确度较低。

### 优先级：P0

方式 B 可以作为临时方案，不阻塞 SDK 开发。但方式 A 是长期正确做法。

---

## 改动 3（建议）：Agent 发送消息时带身份标识

### 当前状态

Bot 进程通过 `sendMessage` API 向群聊发送消息，所有消息都显示为同一个 Bot 账号。

### 问题

一个群里有 2 个 Agent（林晚晴 + 运势大师），但它们共享同一个 Bot Token，发出的消息在 UI 上看起来都是同一个"Bot"在说话，用户分不清谁是谁。

### 解决方案

**方案 A（简单）**：SDK 在消息前加名字前缀

```
林晚晴：嗨～很高兴认识大家
运势大师：让我帮你抽一张塔罗牌...
```

SDK 已经这样做了（`GroupReply.AgentName + ": " + Content`）。不需要平台改动。

**方案 B（更好）**：平台支持多 Bot Identity

每个 Agent 有独立的头像和名字，发消息时指定身份：

```json
{
  "chat_id": "group_123",
  "text": "嗨～很高兴认识大家",
  "sender_identity": {
    "display_name": "林晚晴",
    "avatar_url": "https://..."
  }
}
```

平台在 UI 上展示为不同的"虚拟用户"在发消息。

### 优先级：P2

方案 A 已可用，方案 B 是体验升级。

---

## 改动 4（建议）：Agent Typing Indicator

### 需求

当 Agent 正在思考/生成回复时，群聊 UI 显示"林晚晴正在输入..."。

### 实现方式

Bot 进程在调用 LLM 之前，发送 `sendChatAction` API（Telegram 标准）：

```json
{"chat_id": "group_123", "action": "typing"}
```

### 当前状态

Zapry 目前不支持 `sendChatAction`（在 compat.go 的不支持列表中）。

### 优先级：P3

锦上添花。没有也不影响核心功能。

---

## 改动 5（未来）：Agent 市场与群聊集成

### 需求

用户在群聊中可以直接搜索和邀请平台上的公开 Agent：

1. 用户在群设置中点击"添加 AI 助手"
2. 展示 Agent 市场（按技能/分类筛选）
3. 选择一个 Agent → 自动加入群聊
4. Agent 自动发送自我介绍

### 依赖

- Agent Marketplace（Stage 4）
- AgentCardPublic 的 Visibility 和 Skills 字段
- SDK 的 GroupChat.AddAgent() API

### 优先级：P4（Stage 4）

---

## 不需要平台改动的部分

以下能力完全在 SDK 内部实现，不需要 Zapry 平台配合：

| 能力 | SDK 模块 | 说明 |
|------|---------|------|
| 消息路由（谁回复） | GroupChatRouter | 4 层路由逻辑 |
| 发言策略（cooldown） | SpeakingPolicy | 30s 冷却 + 单回复限制 |
| 共享上下文 | SharedContext | 群消息历史管理 |
| 成员介绍注入 | AgentIntroduction | 自动生成成员描述 |
| Agent 记忆 | MemorySession | 每个 Agent 独立记忆 |
| 自然对话 | NaturalConversation | 情绪检测 + 风格控制 |
| Persona 时间感知 | PersonaTick | 心情 + 活动状态 |

---

## 实施建议

| 阶段 | 平台改动 | SDK 状态 |
|------|---------|---------|
| 现在 | 无改动 | SDK 可用 mock 测试群聊 |
| Phase 1 | 改动 1（群成员类型）+ 改动 2（确认 Webhook 字段） | SDK 对接真实群聊 |
| Phase 2 | 改动 3 方案 B（多 Bot Identity）| 体验升级 |
| Phase 3 | 改动 4（Typing Indicator）| 锦上添花 |
| Phase 4 | 改动 5（Agent 市场集成）| 生态化 |
