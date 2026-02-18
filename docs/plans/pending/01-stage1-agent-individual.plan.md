---
name: Stage 1 Agent Individual
overview: Stage 1：完善 Agent 个体能力，让每个 Agent 在群聊中"像真人"。三件事：Persona Engine 集成进 SDK、AgentLoop 加 Tool Loop Detection、AgentCard 扩展为社交名片 AgentProfile。
todos:
  - id: loop-detector
    content: "Go: loop_detector.go + AgentLoop 集成 + 5 测试"
    status: pending
  - id: agent-profile
    content: "Go: agent_profile.go + AgentRuntimeConfig 扩展 + 3 测试"
    status: pending
  - id: persona-bridge
    content: "Go: persona_bridge.go + PersonaAgentLoop + 4 测试"
    status: pending
  - id: py-stage1
    content: "Python: 翻译 3 个文件 + 12 测试"
    status: pending
  - id: readme-push-s1
    content: README + 提交推送
    status: pending
isProject: false
---

# Stage 1：让每个 Agent 都"像真人"

## 三件事

### 1. Tool Loop Detection（AgentLoop 工具调用死循环检测）

**问题**：LLM 有时会反复用相同参数调同一个工具（查不到的数据、网络超时重试等），白白消耗 turns 和 token。

**在 [agent_loop.go](agent_loop.go) 中新增**：

```go
type LoopDetectorConfig struct {
    Enabled             bool
    MaxRepeatCalls      int // 同一工具+同一参数连续调用上限，默认 3
    MaxSameToolInWindow int // 同一工具在最近 N 次调用中出现的上限，默认 5
    WindowSize          int // 检测窗口大小，默认 10
}

type LoopDetector struct {
    config  LoopDetectorConfig
    history []toolCallEntry // 最近 N 次工具调用记录
}

type toolCallEntry struct {
    Name     string
    ArgsHash string // sha256(json(args))[:16]
}

func (d *LoopDetector) Check(name string, args map[string]interface{}) *LoopWarning
func (d *LoopDetector) Record(name string, args map[string]interface{})
```

检测三种模式（和 OpenClaw 对齐）：

- **genericRepeat**：连续 N 次相同 tool + 相同 args → 终止并返回错误
- **sameToolFlood**：最近窗口内同一 tool 超过阈值 → 注入警告 prompt
- **pingPong**：A/B/A/B 交替模式 → 注入警告 prompt

**集成到 AgentLoop**：在 `RunContext` 的工具执行循环中，每次执行前调 `detector.Check()`，执行后调 `detector.Record()`。

**AgentLoop 新增字段**：

```go
type AgentLoop struct {
    // ... 现有字段 ...
    LoopDetector *LoopDetector // 可选，nil = 不检测
}
```

**StoppedReason 新增**：`"loop_detected"` — 当死循环被终止时。

**测试（5 个）**：

- `TestLoopDetector_RepeatSameArgs` — 连续 3 次相同 → 检测到
- `TestLoopDetector_DifferentArgs_OK` — 同工具不同参数 → 不触发
- `TestLoopDetector_SameToolFlood` — 窗口内过多 → 检测到
- `TestLoopDetector_PingPong` — A/B/A/B 模式 → 检测到
- `TestAgentLoop_LoopDetected_StopsEarly` — 集成测试：死循环 → StoppedReason="loop_detected"

---

### 2. AgentProfile（社交名片，扩展 AgentCard）

**问题**：`AgentCard` 只有技术元数据（agent_id, skills, safety_level）。在社交平台上，Agent 需要给人类看的"社交身份"。

**新增 [agent_profile.go**](agent_profile.go)：

```go
// AgentProfile is the public-facing social identity of an Agent.
// AgentCard is for machine-to-machine (permissions/routing).
// AgentProfile is for human-facing (display in chat/group/marketplace).
type AgentProfile struct {
    AgentID     string `json:"agent_id"`

    // Display info (shown to humans)
    DisplayName string `json:"display_name"`
    Avatar      string `json:"avatar,omitempty"`      // URL
    Bio         string `json:"bio,omitempty"`          // 一句话介绍
    Greeting    string `json:"greeting,omitempty"`     // 首次见面的开场白策略

    // Personality (from Persona Engine, if available)
    PersonalityCode  string   `json:"personality_code,omitempty"`  // MBTI code
    Traits           []string `json:"traits,omitempty"`            // 3-5 个性格标签
    Tone             string   `json:"tone,omitempty"`              // warm/calm/sharp/playful
    RelationshipStyle string  `json:"relationship_style,omitempty"` // friend/mentor/playful/listener

    // Capabilities (for discovery)
    Skills      []string `json:"skills,omitempty"`
    Languages   []string `json:"languages,omitempty"`  // zh-CN, en-US
    Availability string  `json:"availability,omitempty"` // always / business_hours / custom
}

// AgentPresenceStatus represents the Agent's current state in a chat.
type AgentPresenceStatus struct {
    AgentID  string `json:"agent_id"`
    Status   string `json:"status"`   // online / thinking / typing / away / offline
    LastSeen string `json:"last_seen,omitempty"` // RFC3339
}
```

**与现有 AgentCard 的关系**：AgentCard 是**内部治理用的**（权限、路由、安全等级），AgentProfile 是**对外展示用的**。两者通过 `AgentID` 关联。

**AgentRuntimeConfig 扩展**：

```go
type AgentRuntimeConfig struct {
    Card         AgentCardPublic
    Profile      *AgentProfile    // 新增
    // ... 现有字段 ...
}
```

**测试（3 个）**：

- `TestAgentProfile_Serialization` — JSON 序列化/反序列化
- `TestAgentProfile_WithPersona` — 从 PersonaSpec 填充 Profile
- `TestAgentPresenceStatus` — 状态切换

---

### 3. Persona Engine 集成进 SDK（轻量集成，不引入硬依赖）

**问题**：Persona Engine 在独立仓库 `ai-persona-engine`，开发者需要手动集成。SDK 应该提供一个开箱即用的桥接层。

**方案**：不 import persona-engine 的包（避免 go.mod 依赖），而是定义一个**接口**，让开发者或平台层提供实现。

**新增 [persona_bridge.go**](persona_bridge.go)：

```go
// PersonaProvider is the interface for persona compilation and ticking.
// SDK defines the contract; ai-persona-engine provides the implementation.
type PersonaProvider interface {
    // Compile compiles a persona spec into a runtime config.
    Compile(spec map[string]interface{}) (*PersonaRuntimeConfig, error)

    // Tick generates context injection for the current moment.
    Tick(personaID string, userID string, now time.Time) (*PersonaTickResult, error)

    // Get retrieves a compiled persona config.
    Get(personaID string) (*PersonaRuntimeConfig, error)
}

// PersonaRuntimeConfig is the SDK's view of a compiled persona.
type PersonaRuntimeConfig struct {
    PersonaID    string                 `json:"persona_id"`
    SystemPrompt string                 `json:"system_prompt"`
    ModelParams  map[string]interface{} `json:"model_params,omitempty"`
    StylePolicy  map[string]interface{} `json:"style_policy,omitempty"`
    Profile      *AgentProfile          `json:"profile,omitempty"` // auto-filled from spec
}

// PersonaTickResult is the per-turn context injection from persona.
type PersonaTickResult struct {
    PromptInjection      string                 `json:"prompt_injection"`
    StyleConstraintsText string                 `json:"style_constraints_text"`
    ModelParamOverride   map[string]interface{} `json:"model_param_override,omitempty"`
}
```

**PersonaAgentLoop — 把 Persona + NaturalConversation + AgentLoop 编排在一起**：

```go
// PersonaAgentLoop combines Persona Engine + NaturalConversation + AgentLoop
// into a single "make it human-like" entry point.
type PersonaAgentLoop struct {
    inner    *AgentLoop
    nc       *NaturalConversation
    persona  PersonaProvider
    personaID string
}

func NewPersonaAgentLoop(
    loop *AgentLoop,
    persona PersonaProvider,
    personaID string,
    ncConfig ...NaturalConversationConfig,
) *PersonaAgentLoop

// Run executes: PersonaTick → NaturalConversation.Enhance → AgentLoop.Run → PostProcess
func (pl *PersonaAgentLoop) Run(
    session *MemorySession,
    userInput string,
    history []map[string]interface{},
) *AgentLoopResult

func (pl *PersonaAgentLoop) RunContext(
    ctx context.Context,
    session *MemorySession,
    userInput string,
    history []map[string]interface{},
) *AgentLoopResult
```

**开发者使用**：

```go
// 1. 创建 Persona Provider（由 ai-persona-engine 实现，或 mock）
provider := myPersonaProvider // 实现 PersonaProvider 接口

// 2. 编译 persona
config, _ := provider.Compile(map[string]interface{}{
    "name": "林晚晴",
    "traits": []string{"温柔", "理性"},
    "relationship_style": "friend",
    "personality_code": "INTJ",
})

// 3. 创建 PersonaAgentLoop（一行代码获得全部"像人"能力）
pLoop := agentsdk.NewPersonaAgentLoop(loop, provider, config.PersonaID)

// 4. 每次对话
result := pLoop.Run(session, userInput, history)
// 自动：PersonaTick → 状态追踪 → 情绪检测 → 风格修正 → 开场策略
```

**测试（4 个）**：

- `TestPersonaAgentLoop_WithMockProvider` — mock provider → Tick 注入生效
- `TestPersonaAgentLoop_PostProcess` — 风格修正在 persona 模式下仍生效
- `TestPersonaAgentLoop_NilProvider_Fallback` — 无 provider → 退化为 NaturalAgentLoop
- `TestPersonaRuntimeConfig_ToProfile` — config 自动生成 AgentProfile

---

## 文件清单


| 文件                       | 内容                                    | 行数         |
| ------------------------ | ------------------------------------- | ---------- |
| `loop_detector.go`       | LoopDetector + 3 种检测模式                | ~150       |
| `agent_profile.go`       | AgentProfile + AgentPresenceStatus    | ~80        |
| `persona_bridge.go`      | PersonaProvider 接口 + PersonaAgentLoop | ~180       |
| `loop_detector_test.go`  | 5 测试                                  | ~150       |
| `agent_profile_test.go`  | 3 测试                                  | ~60        |
| `persona_bridge_test.go` | 4 测试                                  | ~120       |
| **总计**                   |                                       | **~740 行** |


Python SDK 同步翻译（估计 ~600 行）。

---

## 实施顺序

1. `loop_detector.go` + AgentLoop 集成 + 5 测试（1 天）
2. `agent_profile.go` + AgentRuntimeConfig 扩展 + 3 测试（0.5 天）
3. `persona_bridge.go` + PersonaAgentLoop + 4 测试（1 天）
4. Python SDK 翻译（0.5 天）
5. README 更新 + 提交推送（0.5 天）

**总计 3.5 天**，交付后：开发者一行代码就能创建一个"有性格、有记忆、有情绪、不会死循环、有社交名片"的 Agent。