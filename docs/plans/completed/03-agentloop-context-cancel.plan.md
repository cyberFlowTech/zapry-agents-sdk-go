---
name: AgentLoop Context Cancel
overview: 为 Go + Python SDK 的 AgentLoop 添加 context.Context 支持，使 LLM 调用和工具执行可被取消（用户追问时中断旧回复），同时新增 RunContext/run_with_context 方法保持向后兼容。
todos:
  - id: go-runcontext
    content: "Go: agent_loop.go — LLMFuncWithContext + RunContext 方法 + Run 委托"
    status: completed
  - id: go-runcontext-test
    content: "Go: agent_loop_test.go — 4 个新测试"
    status: completed
  - id: py-runcancel
    content: "Python: agent/loop.py — run_with_cancel + _run_inner 抽取"
    status: completed
  - id: py-runcancel-test
    content: "Python: test_agent_loop.py — 3 个新测试"
    status: completed
  - id: commit-push
    content: 提交 + 推送两个 SDK
    status: completed
isProject: false
---

# AgentLoop Context/Cancel 支持

## 解决的核心问题

用户连续追问时，上一次 AgentLoop.Run 可能仍在执行（等 LLM 响应 / 执行工具），新消息进来后旧回复最终才产出，造成消息乱序。通过 context 支持，业务层可在新消息到来时取消旧的 Run，立即释放资源。

## 设计原则

- **向后兼容**：现有 `Run()` 签名不变，新增 `RunContext(ctx, ...)` 方法
- **穿透传播**：ctx 从 RunContext → LLMFunc → ToolRegistry.Execute → ToolContext.Ctx → MCP CallTool，全链路可取消
- **StoppedReason 新增 "cancelled"**：区分正常完成 / 超时 / 主动取消
- **两个 SDK 对齐**：Go 和 Python 同步实现

---

## Go SDK 改动

### 1. LLMFunc 签名升级（向后兼容）

新增 `LLMFuncWithContext`，同时保留旧的 `LLMFunc`：

```go
// 新签名（推荐）
type LLMFuncWithContext func(ctx context.Context, messages []map[string]interface{}, tools []map[string]interface{}) (*LLMMessage, error)
```

AgentLoop 内部：如果设置了 `LLMFnCtx`，优先用；否则包装旧的 `LLMFn` 忽略 ctx。

### 2. AgentLoop 结构体新增字段

在 [agent_loop.go](agent_loop.go) 第 70-78 行的 `AgentLoop` struct 中：

```go
type AgentLoop struct {
    LLMFn        LLMFunc              // 保留，向后兼容
    LLMFnCtx     LLMFuncWithContext   // 新增，支持 context 的 LLM 调用
    ToolRegistry *ToolRegistry
    SystemPrompt string
    MaxTurns     int
    Hooks        *AgentLoopHooks
    Guardrails   *GuardrailManager
    Tracer       *AgentTracer
}
```

### 3. 新增 RunContext 方法

```go
func (a *AgentLoop) RunContext(ctx context.Context, userInput string, conversationHistory []map[string]interface{}, extraContext string) *AgentLoopResult
```

内部改动（基于现有 Run 的第 98-339 行）：

- 循环开头检查 `ctx.Err()`，如果已取消则 `StoppedReason = "cancelled"` 并 break
- LLM 调用前检查 ctx：`select { case <-ctx.Done(): ... }`
- 调用 LLM 时传 ctx（如果 `LLMFnCtx != nil`）
- 创建 ToolContext 时传入 ctx（第 279 行已经有 `Ctx: context.Background()`，改为传入外层 ctx）
- 每个工具执行前检查 ctx

现有 `Run()` 方法改为调用 `RunContext(context.Background(), ...)`，零影响。

### 4. AgentLoopResult.StoppedReason 新增值

现有值：`"completed"`, `"max_turns"`, `"error"`, `"guardrail"`

新增：`"cancelled"` — 当 ctx 被取消时

### 5. 改动文件

- [agent_loop.go](agent_loop.go) — 新增 `LLMFuncWithContext` 类型 + `RunContext` 方法 + `Run` 委托给 `RunContext`
- [agent_loop_test.go](agent_loop_test.go) — 新增 4 个测试

### 6. 不改动的文件

- `tools.go` — ToolContext.Ctx 已经在 MCP 阶段加好了
- `mcp_*.go` — MCP 已经全链路支持 ctx

---

## Python SDK 改动

### 1. AgentLoop 新增 run_with_context

Python 天然 async，用 `asyncio` 的取消机制：

在 [zapry_agents_sdk/agent/loop.py](../zapry-agents-sdk-python/zapry_agents_sdk/agent/loop.py) 中：

```python
async def run(self, user_input, conversation_history=None, extra_context=None) -> AgentResult:
    """原方法不变，向后兼容。"""
    return await self._run_inner(user_input, conversation_history, extra_context)

async def run_with_cancel(self, cancel_event: asyncio.Event, user_input, ...) -> AgentResult:
    """支持外部取消的 run。当 cancel_event 被 set 时，循环中断。"""
```

内部改动：

- 每轮循环开头检查 `cancel_event.is_set()`
- LLM 调用用 `asyncio.shield` 或在调用后检查 cancel
- 工具执行前检查 cancel
- 取消时 `stopped_reason = "cancelled"`

### 2. 改动文件

- `zapry_agents_sdk/agent/loop.py` — 新增 `run_with_cancel` + `_run_inner` 抽取
- `tests/test_agent_loop.py` — 新增 3 个测试

---

## 使用示例

### Go SDK

```go
// 业务层：管理每个会话的 cancel
var mu sync.Mutex
var activeRuns = map[string]context.CancelFunc{} // chatID -> cancel

func handleMessage(bot *agentsdk.AgentAPI, update agentsdk.Update) {
    chatID := update.Message.Chat.ID

    // 取消该会话上一次 run
    mu.Lock()
    if cancel, ok := activeRuns[chatID]; ok {
        cancel()
    }
    ctx, cancel := context.WithCancel(context.Background())
    activeRuns[chatID] = cancel
    mu.Unlock()

    defer func() {
        mu.Lock()
        delete(activeRuns, chatID)
        mu.Unlock()
    }()

    result := loop.RunContext(ctx, update.Message.Text, nil, "")

    if result.StoppedReason == "cancelled" {
        return // 被新消息取消，不发送
    }
    bot.Send(agentsdk.NewMessage(chatID, result.FinalOutput))
}
```

### Python SDK

```python
active_runs: dict[str, asyncio.Event] = {}

async def handle_message(update, context):
    chat_id = str(update.message.chat.id)

    # 取消旧 run
    if chat_id in active_runs:
        active_runs[chat_id].set()

    cancel_event = asyncio.Event()
    active_runs[chat_id] = cancel_event

    try:
        result = await loop.run_with_cancel(cancel_event, update.message.text)
        if result.stopped_reason == "cancelled":
            return
        await update.message.reply_text(result.final_output)
    finally:
        active_runs.pop(chat_id, None)
```

---

## 测试矩阵

### Go: agent_loop_test.go 新增 4 个

- `TestRunContext_Cancelled_BeforeLLM` — ctx 在 LLM 调用前取消 → StoppedReason="cancelled"
- `TestRunContext_Cancelled_DuringToolExec` — ctx 在工具执行中取消 → 停止后续工具调用
- `TestRunContext_WithTimeout` — context.WithTimeout 超时 → StoppedReason="cancelled"
- `TestRunContext_BackwardsCompat` — Run() 行为完全不变（委托给 RunContext + Background）

### Python: test_agent_loop.py 新增 3 个

- `test_run_with_cancel_before_llm` — cancel_event set 后 → stopped_reason="cancelled"
- `test_run_with_cancel_during_tool` — 工具执行中取消
- `test_run_backwards_compat` — run() 不受影响

