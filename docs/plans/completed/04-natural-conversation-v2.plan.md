---
name: Natural Conversation v2
overview: 自然对话 SDK v2.1（对外发布版）：采纳全部产品建议 + 6 条发布级修正。WorkingMemory 原子操作确保并发安全；PromptFragments 加 Warnings 调试字段；StyleController 截断加自然收尾句；EmotionDetector 中英双语 + 反例保护 + 差异化权重；Compressor 可插拔 EstimateTokensFn + 摘要 tag；WrapLoop + Enhance/PostProcess + Hooks 三种接入方式。
todos:
  - id: go-working-memory
    content: "Go: memory_layers.go WorkingMemory 新增 GetInt/SetInt/Incr/GetString/SetString 原子操作"
    status: completed
  - id: go-fragments
    content: "Go: prompt_fragments.go — PromptFragments + KV + Warnings"
    status: completed
  - id: go-state
    content: "Go: conversation_state.go + 7 测试"
    status: completed
  - id: go-emotion
    content: "Go: emotional_tone.go + 7 测试（中英双语 + 反例保护 + 差异化权重）"
    status: completed
  - id: go-style
    content: "Go: response_style.go + 7 测试（PostProcess 本地修正 + 自然收尾句 + 最小保留）"
    status: completed
  - id: go-opener
    content: "Go: conversation_opener.go + 6 测试（含频率限制）"
    status: completed
  - id: go-compressor
    content: "Go: context_compressor.go + 6 测试（可插拔 EstimateTokensFn + 摘要 tag + 版本）"
    status: completed
  - id: go-natural
    content: "Go: natural_conversation.go + 5 集成测试（Enhance + PostProcess + WrapLoop + Hooks模式 + 默认配置）"
    status: completed
  - id: py-all
    content: "Python: 翻译 9 个文件 + 38 测试"
    status: completed
  - id: readme-push
    content: README（Recommended/Advanced 两档 + 三种接入方式说明）+ 提交推送
    status: completed
isProject: false
---

# 自然对话 SDK v2.1 — 对外发布版

## v2 → v2.1 发布级修正（6 条）

1. **WorkingMemory 原子操作**：新增 `GetInt/SetInt/Incr/GetString/SetString`，确保并发下 turn_index/opener_count/summary_cache 不错乱
2. **PromptFragments 加 Warnings**：`Warnings []string` 记录每步决策原因（如 `"style.truncated:exceeded_300"`, `"tone.anxious:followup_boost"`），给开发者调试用，不注入 LLM
3. **StyleController 截断自然收尾**：截断后追加随机自然收尾句（"先说到这儿。"/"大概就是这样。"），最小保留 40 字不截断
4. **EmotionDetector 中英双语 + 反例保护**：英文核心词（quick/ASAP/bullshit）；happy 权重降为 0.3（需多命中），angry 强词权重 0.5
5. **Compressor 可插拔 EstimateTokensFn**：默认 `runeCount/2.7`，开发者可替换为真实 tokenizer；摘要 system msg 加 `[sdk.summary:v1]` tag
6. **三种接入方式**：WrapLoop（推荐）/ Enhance+PostProcess（手动）/ AgentLoopHooks（钩子模式）

---

## 两档能力分级

### Recommended（默认推荐，零额外 LLM 成本）


| 能力                           | 默认  | 说明                          |
| ---------------------------- | --- | --------------------------- |
| ConversationStateTracker     | ON  | 纯计算，零成本                     |
| EmotionalToneDetector        | ON  | 权重打分，零成本                    |
| StyleController（PostProcess） | ON  | **本地修正**：截断/删禁止词/改问号，不调 LLM |


### Advanced（需显式开启，有 LLM 成本）


| 能力                    | 默认  | 说明                                         |
| --------------------- | --- | ------------------------------------------ |
| ConversationOpener    | OFF | 策略提示注入，零 LLM 成本但需调参                        |
| ContextCompressor     | OFF | 需要 SummarizeFn（1 次 LLM 调用），有 token gate 控制 |
| StyleController Retry | OFF | 不合格时 retry 1 次 LLM，仅高价值场景                  |


---

## 核心结构变更（vs v1）

### 变更 0 (v2.1 新增): WorkingMemory 原子操作

在现有 [memory_layers.go](memory_layers.go) 的 `WorkingMemory` 上新增类型化方法：

```go
func (w *WorkingMemory) GetInt(key string) int       // 不存在返回 0
func (w *WorkingMemory) SetInt(key string, val int)
func (w *WorkingMemory) Incr(key string) int          // 原子 +1 并返回新值
func (w *WorkingMemory) GetString(key string) string   // 不存在返回 ""
func (w *WorkingMemory) SetString(key string, val string)
```

全部在 `w.mu.Lock()` 保护下操作，并发安全。

### 变更 1: PromptFragments 替代 string extra_context

```go
type PromptFragments struct {
    // SystemAdditions 多段策略提示，最终 join("\n\n") 注入 extra_context
    SystemAdditions []string
    // KV 结构化元数据，业务层可读，命名空间 sdk.*
    KV map[string]interface{}
    // Warnings 每步决策记录（调试用，不注入 LLM）
    Warnings []string
}

// Text 返回可注入 LLM 的合并文本。
func (f *PromptFragments) Text() string

// AddWarning 记录一条调试信息。
func (f *PromptFragments) AddWarning(msg string)
```

### 变更 2: StyleController.PostProcess 替代 CheckAndRetry

```go
// PostProcess 本地修正输出（默认启用，不调 LLM）。
func (c *ResponseStyleController) PostProcess(output string) (final string, changed bool, violations []string)

// BuildRetryPrompt 高级功能（默认不推荐开启）。
func (c *ResponseStyleController) BuildRetryPrompt(output string, violations []string) string
```

### 变更 3: EmotionalToneDetector 权重打分

```go
type EmotionalTone struct {
    Tone       string             // 最高分情绪
    Confidence float64            // clamp(score, 0, 1)
    Scores     map[string]float64 // 所有情绪得分
}
```

### 变更 4: Opener 频率限制

```go
type OpenerStrategy struct {
    Situation            string
    Hint                 string
    MaxMentionsPerSession int // 默认 1
}
```

### 变更 5: Compressor token gate + 摘要版本

```go
type CompressorConfig struct {
    WindowSize     int    // 最近 N 条完整保留，默认 6
    TokenThreshold int    // 预估 token 超过此值才压缩，默认 6000
    SummaryVersion string // 摘要策略版本号，变更时自动失效缓存
}
```

### 变更 6: ConversationState 补两个字段

```go
type ConversationState struct {
    // ... 原有字段 ...
    IsFirstConversation bool   // DaysSinceLast==-1 的语义化封装
    LocalTime           string // RFC3339 带时区
}
```

---

## 文件规划（Go SDK，9 个文件）


| 文件                             | 内容                                                                             | 行数   |
| ------------------------------ | ------------------------------------------------------------------------------ | ---- |
| `memory_layers.go` (修改)        | WorkingMemory 新增 GetInt/SetInt/Incr/GetString/SetString                        | +30  |
| `prompt_fragments.go`          | PromptFragments + KV + Warnings                                                | ~80  |
| `conversation_state.go`        | ConversationStateTracker + ConversationState（含 IsFirstConversation, LocalTime） | ~180 |
| `emotional_tone.go`            | EmotionalToneDetector（中英双语 + 差异化权重 + 反例保护）                                     | ~200 |
| `response_style.go`            | StyleConfig + PostProcess 本地修正（自然收尾 + 最小保留） + BuildRetryPrompt                 | ~250 |
| `conversation_opener.go`       | OpenerGenerator + 频率限制（MaxMentionsPerSession）                                  | ~140 |
| `context_compressor.go`        | ContextCompressor + EstimateTokensFn + token gate + 摘要 tag + 版本                | ~250 |
| `natural_conversation.go`      | NaturalConversation + Enhance + PostProcess + WrapLoop + BuildHooks            | ~200 |
| `natural_conversation_test.go` | 全量测试 ~38 个                                                                     | ~700 |


---

## 能力 1: ConversationStateTracker

### 接口

```go
type ConversationState struct {
    TurnIndex           int
    IsFollowUp          bool
    IsFirstConversation bool          // 语义化（DaysSinceLast==-1）
    SessionDuration     time.Duration
    DaysSinceLast       int           // -1=首次
    TotalSessions       int
    TimeOfDay           string        // morning/afternoon/evening/late_night
    UserMsgLength       string        // short/medium/long
    LocalTime           string        // RFC3339 带时区
}

type ConversationStateTracker struct {
    followUpWindow time.Duration // 默认 60s
    timezone       *time.Location
}

func NewConversationStateTracker(timezone ...string) *ConversationStateTracker

func (t *ConversationStateTracker) Track(session *MemorySession, userInput string, now time.Time) *ConversationState

// FormatForPrompt 返回策略提示文本。
func (s *ConversationState) FormatForPrompt() string

// ToKV 返回结构化 map（命名空间 sdk.conversation.* / sdk.session.* / sdk.user.*）。
func (s *ConversationState) ToKV() map[string]interface{}
```

`ToKV` 输出的 key：

- `sdk.conversation.days_since_last`
- `sdk.conversation.total_sessions`
- `sdk.conversation.is_first`
- `sdk.session.turn_index`
- `sdk.session.duration_sec`
- `sdk.user.is_followup`
- `sdk.user.msg_length`
- `sdk.runtime.time_of_day`
- `sdk.runtime.local_time`

持久化：`total_sessions` 和 `last_at` 存 `MemoryStore`（key=`sdk.conversation_meta`）。`turn_index` 存 `WorkingMemory`。

### 测试（7 个）

- `TestTrack_FirstConversation` — IsFirstConversation=true, DaysSinceLast=-1
- `TestTrack_DaysSinceLast` — 3 天前 → 3
- `TestTrack_IsFollowUp` — 30s 内 → true, 90s 后 → false
- `TestTrack_TurnIndex` — 每次 +1
- `TestTrack_TimeOfDay` — 各时段
- `TestTrack_LocalTime_WithTimezone` — 指定时区输出正确
- `TestTrack_ToKV_Namespace` — 所有 key 以 sdk. 开头

---

## 能力 2: EmotionalToneDetector（权重打分）

### 接口

```go
type EmotionalTone struct {
    Tone       string             // 最高分情绪（neutral/anxious/angry/happy/sad）
    Confidence float64            // clamp(maxScore, 0, 1)
    Scores     map[string]float64 // 所有情绪得分明细
}

type EmotionalToneDetector struct {
    patterns map[string][]weightedKeyword // tone -> keywords with weight
}

type weightedKeyword struct {
    keyword string
    weight  float64
}

func NewEmotionalToneDetector() *EmotionalToneDetector

// Detect 检测用户消息情绪（可传入 ConversationState 辅助判断）。
func (d *EmotionalToneDetector) Detect(userInput string, state *ConversationState) *EmotionalTone

// FormatForPrompt 生成情绪提示（neutral 或 confidence<0.3 返回空字符串）。
func (t *EmotionalTone) FormatForPrompt() string
```

打分规则（v2.1 差异化权重）：

- **angry 强词命中**：+0.5（"什么破"/"垃圾"/"bullshit"）
- **anxious/sad 关键词命中**：+0.4
- **happy 关键词命中**：+0.3（降低，减少"哈哈"阴阳怪气误判）
- `IsFollowUp=true` + `UserMsgLength="short"`：anxious +0.2
- 多个感叹号（>=2）：当前最高情绪 +0.1（上限 +0.2）
- 最终 `Tone = max(scores)`，`Confidence = clamp(maxScore, 0, 1)`

中英双语内置关键词（示例）：


| 情绪      | 中文         | 英文                            | 默认权重 |
| ------- | ---------- | ----------------------------- | ---- |
| angry   | 什么破、垃圾、搞什么 | bullshit, wtf, terrible       | 0.5  |
| anxious | 快点、赶紧、急    | quick, ASAP, hurry            | 0.4  |
| happy   | 太好了、棒、开心   | awesome, great, love it       | 0.3  |
| sad     | 唉、算了、难过    | sigh, forget it, disappointed | 0.4  |


输出节制：

- `Tone=="neutral"` 或 `Confidence < 0.3` → `FormatForPrompt()` 返回 `""`
- 提示措辞**温和间接**："用户语气偏急促，注意回复节奏"，**不说**"用户在生气/焦虑"

### 测试（7 个）

- `TestDetect_Anxious_Chinese` — "快点" → anxious >=0.4
- `TestDetect_Angry_StrongWord` — "什么破东西" → angry >=0.5
- `TestDetect_Happy_LowWeight` — 单个"哈哈" → happy 0.3（不够 0.3 阈值，不输出）
- `TestDetect_Happy_MultiHit` — "太好了哈哈棒" → happy >=0.6（多命中才高）
- `TestDetect_English` — "ASAP please" → anxious
- `TestDetect_FollowUpBoost` — followup+short → anxious +0.2
- `TestDetect_Neutral_NoOutput` — 普通消息 → ""

---

## 能力 3: ResponseStyleController（本地修正优先）

### 接口

```go
type StyleConfig struct {
    MaxLength        int      // rune 上限，默认 300，0=不限
    PreferredLength  int      // 注入 prompt 的建议长度，默认 150
    ForbiddenPhrases []string // 禁止词列表
    EndStyle         string   // "no_question" / ""
    EnableRetry      bool     // 默认 false（高级功能）
}

func DefaultStyleConfig() StyleConfig

type ResponseStyleController struct {
    config StyleConfig
}

func NewResponseStyleController(config ...StyleConfig) *ResponseStyleController

// BuildStylePrompt 生成风格约束的 prompt 片段（注入 system prompt）。
func (c *ResponseStyleController) BuildStylePrompt() string

// PostProcess 本地修正（默认启用，不调 LLM）。
// 返回：修正后文本、是否改动、违规项列表。
func (c *ResponseStyleController) PostProcess(output string) (string, bool, []string)

// BuildRetryPrompt 生成重试 prompt（高级，EnableRetry=true 时才用）。
func (c *ResponseStyleController) BuildRetryPrompt(output string, violations []string) string
```

PostProcess 本地修正内容（v2.1 增强）：

1. **超长截断**：
  - **最小保留 40 字**：不足 40 字不截断（即使超过 MaxLength）
  - 截断到 MaxLength，在最近的句号/换行处截断（不在字中间断）
  - **追加随机自然收尾句**（不是 "…"）：从列表中随机选一句
    - `"先说到这儿。"` / `"大概就是这样。"` / `"就先聊这些吧。"` / `"回头再细说。"`
  - 记录 Warning：`"style.truncated:exceeded_{MaxLength}"`
2. **删除禁止词**：直接从文本中移除整个匹配的短语，记录 `"style.forbidden_removed:{phrase}"`
3. **结尾问号改句号**：如果 `EndStyle="no_question"` 且以 `？`/`?` 结尾，改为 `。`/`.`
4. **删除套话连接词**（可选）：去掉"首先，"/"其次，"/"总之，"开头

默认 ForbiddenPhrases：

```go
[]string{
    "作为一个AI", "作为AI助手", "作为一个人工智能",
    "我是一个AI", "我是AI助手",
    "有什么我可以帮你的", "还有什么需要帮忙的",
    "请问还有什么", "很高兴为你服务",
    "希望对你有帮助", "如果你有任何问题",
}
```

### 测试（7 个）

- `TestPostProcess_TooLong_NaturalEnding` — 超 MaxLength → 截断到句号 + 自然收尾句（不是 "…"）
- `TestPostProcess_MinPreserve_NoTruncate` — 35 字文本 MaxLength=30 → 不截断（最小保留 40）
- `TestPostProcess_ForbiddenPhrase_Removed` — 禁止词被删除
- `TestPostProcess_EndQuestion_Fixed` — ？→ 。
- `TestPostProcess_Normal_NoChange` — 正常文本不改动
- `TestPostProcess_Warnings_Recorded` — 截断+禁止词 → Warnings 包含原因
- `TestBuildStylePrompt` — 包含 PreferredLength 提示

---

## 能力 4: ConversationOpener（含频率限制）

### 接口

```go
type OpenerStrategy struct {
    Situation string // first_meeting/long_absence/followup/late_night/normal
    Hint      string
}

type OpenerConfig struct {
    MaxMentionsPerSession int // 默认 1（每 session 最多注入 1 次 opener）
    LongAbsenceDays       int // 默认 3（>=3 天算 long_absence）
}

type OpenerGenerator struct {
    config OpenerConfig
}

func NewOpenerGenerator(config ...OpenerConfig) *OpenerGenerator

// Generate 根据 ConversationState 生成开场策略。
// sessionOpenerCount 是本 session 已注入过的次数。
func (g *OpenerGenerator) Generate(state *ConversationState, sessionOpenerCount int) *OpenerStrategy
```

频率限制：如果 `sessionOpenerCount >= MaxMentionsPerSession` → 返回 `Situation="normal"`（不注入 hint）。

`NaturalConversation` 内部用 `WorkingMemory` 存 `sdk.opener_count`，每次 Generate 后 +1。

### 测试（6 个）

- `TestOpener_FirstMeeting`
- `TestOpener_LongAbsence` — 7 天 → long_absence
- `TestOpener_FollowUp` — IsFollowUp → followup
- `TestOpener_LateNight`
- `TestOpener_FrequencyLimit` — 第 2 次调用 → normal
- `TestOpener_Normal`

---

## 能力 5: ContextCompressor（token gate + 摘要版本）

### 接口

```go
type SummarizeFn func(messages []map[string]interface{}) (string, error)

// EstimateTokensFn 预估 token 数量。可插拔，开发者可替换为真实 tokenizer。
type EstimateTokensFn func(history []map[string]interface{}) int

type CompressorConfig struct {
    WindowSize       int              // 最近 N 条完整保留，默认 6
    TokenThreshold   int              // 预估 token 超过此值才压缩，默认 6000
    SummaryVersion   string           // 策略版本号，变更时缓存自动失效，默认 "v1"
    EstimateTokensFn EstimateTokensFn // 可插拔 token 估算器，默认 runeCount/2.7
}

type ContextCompressor struct {
    config      CompressorConfig
    summarizeFn SummarizeFn
}

func NewContextCompressor(fn SummarizeFn, config ...CompressorConfig) *ContextCompressor

// Compress 压缩对话历史。
// 1. 如果预估 token < TokenThreshold → 原样返回
// 2. 否则调 SummarizeFn 生成摘要 + 最近 WindowSize 条
// 摘要消息带 tag：[sdk.summary:{version}]
// 缓存在 working memory（key=sdk.context_summary:{version}）
func (c *ContextCompressor) Compress(
    history []map[string]interface{},
    working *WorkingMemory,
) ([]map[string]interface{}, error)
```

Token 估算（默认实现）：`runeCount(allContent) / 2.7`（比 /3 更接近实际）+ 对 code block 内容额外 x1.5。开发者可通过 `EstimateTokensFn` 替换为真实 tokenizer。

摘要 system msg 格式：`[sdk.summary:v1] 用户目标：...\n已提供信息：...\n未解决问题：...`（方便调试/回放）。

摘要格式约束（SummarizeFn 的 prompt 里固定）：

```
请用结构化列表总结以下对话中的关键信息：
- 用户目标/需求
- 用户已提供的关键信息
- 未解决的问题
- Agent 的承诺/结论
不要添加评价或艺术化描述。
```

### 测试（6 个）

- `TestCompress_BelowThreshold_NoChange` — token 不够 → 原样
- `TestCompress_AboveThreshold_Summarized` — 超过 → 摘要 + 最近 N 条
- `TestCompress_SummaryHasTag` — 摘要 msg 包含 `[sdk.summary:v1]` tag
- `TestCompress_CacheHit` — 第二次不重新调 SummarizeFn
- `TestCompress_VersionChange_CacheInvalidated` — 版本号变 → 重新生成
- `TestCompress_CustomEstimator` — 自定义 EstimateTokensFn 生效

---

## 统一入口: NaturalConversation

### 接口

```go
type NaturalConversationConfig struct {
    // Recommended（默认 ON）
    StateTracking    bool // 默认 true
    EmotionDetection bool // 默认 true
    StylePostProcess bool // 默认 true

    // Advanced（默认 OFF）
    OpenerGeneration bool // 默认 false
    ContextCompress  bool // 默认 false
    StyleRetry       bool // 默认 false

    // 子配置
    StyleConfig      StyleConfig
    OpenerConfig     OpenerConfig
    CompressorConfig CompressorConfig
    SummarizeFn      SummarizeFn // ContextCompress=true 时必须提供
    Timezone         string      // 默认 "Asia/Shanghai"
    FollowUpWindow   time.Duration // 默认 60s
}

func DefaultNaturalConversationConfig() NaturalConversationConfig

type NaturalConversation struct { ... }

func NewNaturalConversation(config NaturalConversationConfig) *NaturalConversation

// Enhance 在 Run 前增强：注入状态/情绪/开场策略，压缩 history。
func (nc *NaturalConversation) Enhance(
    session *MemorySession,
    userInput string,
    history []map[string]interface{},
    now time.Time,
) (*PromptFragments, []map[string]interface{})

// PostProcess 在 Run 后本地修正输出。
func (nc *NaturalConversation) PostProcess(output string) (string, bool)

// BuildRetryPrompt 高级：生成重试 prompt（StyleRetry=true 时才用）。
func (nc *NaturalConversation) BuildRetryPrompt(output string) *string
```

### 三种接入方式（v2.1）

#### 方式 A: WrapLoop（最推荐，一键启用）

```go
nc := agentsdk.NewNaturalConversation(agentsdk.DefaultNaturalConversationConfig())
naturalLoop := nc.WrapLoop(loop)

result := naturalLoop.Run(session, userInput, history)
// 自动：Enhance → inner.Run → PostProcess
// Recommended 默认全开，无额外 LLM 成本
```

```go
type NaturalAgentLoop struct {
    inner *AgentLoop
    nc    *NaturalConversation
}

func (nl *NaturalAgentLoop) Run(session *MemorySession, userInput string, history []map[string]interface{}) *AgentLoopResult

func (nl *NaturalAgentLoop) RunContext(ctx context.Context, session *MemorySession, userInput string, history []map[string]interface{}) *AgentLoopResult

// LastFragments 返回最近一次 Enhance 的 PromptFragments（含 Warnings，用于调试）。
func (nl *NaturalAgentLoop) LastFragments() *PromptFragments
```

#### 方式 B: Enhance + PostProcess 手动调用（高级）

```go
nc := agentsdk.NewNaturalConversation(config)

// Run 前
fragments, enhancedHistory := nc.Enhance(session, userInput, history, time.Now())
// 读 Warnings：fragments.Warnings → ["tone.anxious:followup_boost", "style.prompt:preferred_150"]

result := loop.RunContext(ctx, userInput, enhancedHistory, fragments.Text())

// Run 后
final, changed := nc.PostProcess(result.FinalOutput)
// 读 KV：fragments.KV["sdk.session.turn_index"] → 3
```

#### 方式 C: AgentLoopHooks 钩子模式（不想换 Loop 类型的开发者）

```go
nc := agentsdk.NewNaturalConversation(config)
hooks := nc.BuildHooks(session)

loop.Hooks.OnLLMStart = hooks.OnLLMStart  // 注入 fragments
loop.Hooks.OnTurnEnd = hooks.OnTurnEnd    // PostProcess
```

这样开发者继续用原来的 `AgentLoop`，不需要换成 `NaturalAgentLoop`。

### 高级配置示例

```go
config := agentsdk.DefaultNaturalConversationConfig()
config.OpenerGeneration = true
config.ContextCompress = true
config.CompressorConfig.SummarizeFn = mySummarize
config.CompressorConfig.EstimateTokensFn = myTokenCounter // 可选：自定义 tokenizer
config.StyleRetry = true

nc := agentsdk.NewNaturalConversation(config)
naturalLoop := nc.WrapLoop(loop)
result := naturalLoop.Run(session, userInput, history)

// 调试：查看决策过程
for _, w := range naturalLoop.LastFragments().Warnings {
    log.Println("[NaturalConv]", w)
}
```

---

## 测试总计


| 文件                           | 用例                                                 |
| ---------------------------- | -------------------------------------------------- |
| memory_layers_test.go (追加)   | 3（Incr 原子性 + GetInt/GetString）                     |
| conversation_state_test.go   | 7                                                  |
| emotional_tone_test.go       | 7（中英双语 + 反例保护）                                     |
| response_style_test.go       | 7（自然收尾 + 最小保留 + Warnings）                          |
| conversation_opener_test.go  | 6                                                  |
| context_compressor_test.go   | 6（自定义 estimator + 摘要 tag）                          |
| natural_conversation_test.go | 5（Enhance + PostProcess + WrapLoop + Hooks + 默认配置） |
| **Go 总计**                    | **41**                                             |
| **Python 总计**                | **38**（Go 的 WorkingMemory 原子操作 Python 不需要，已有线程安全）  |


---

## 实施顺序

### Phase 1: Go SDK 基础设施 + Recommended 基线（2 天）

1. `memory_layers.go` 修改 — WorkingMemory 原子操作 + 3 测试
2. `prompt_fragments.go` — PromptFragments + KV + Warnings
3. `conversation_state.go` + 7 测试 — 基础，其他依赖它
4. `emotional_tone.go` + 7 测试 — 中英双语 + 差异化权重
5. `response_style.go` + 7 测试 — PostProcess 本地修正 + 自然收尾

### Phase 2: Go SDK Advanced + 统一入口（1.5 天）

1. `conversation_opener.go` + 6 测试 — 含频率限制
2. `context_compressor.go` + 6 测试 — EstimateTokensFn + 摘要 tag + 版本
3. `natural_conversation.go` + 5 集成测试 — Enhance + PostProcess + WrapLoop + BuildHooks

### Phase 3: Python SDK（1 天）

1. 翻译全部 9 个文件 + 38 测试

### Phase 4: 文档 + 发布（0.5 天）

1. README 更新：
  - Recommended / Advanced 两档说明
    - 三种接入方式（WrapLoop / 手动 / Hooks）示例
    - Warnings 调试说明
2. 提交 + 推送

