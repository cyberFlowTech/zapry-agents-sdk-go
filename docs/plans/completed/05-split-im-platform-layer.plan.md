---
name: Split IM Platform Layer
overview: 将 Go SDK 的 IM 平台通讯层（Telegram/Zapry Bot API）拆分到 platform/telegram/ 子包，AI Agent 框架层留在根包。零交叉依赖，拆分干净。
todos:
  - id: create-dir
    content: 创建 imbotapi/ 目录 + git mv 移动 13 个源文件 + 5 个测试文件
    status: completed
  - id: fix-package
    content: 修改所有移动文件的 package 声明为 imbotapi
    status: completed
  - id: fix-examples
    content: 更新 examples/ 的 import 路径
    status: completed
  - id: verify-build
    content: 编译检查 + 运行全部测试
    status: completed
  - id: update-readme
    content: 更新 README Project Structure
    status: completed
  - id: commit-push
    content: 提交 + 推送
    status: completed
isProject: false
---

# Go SDK 结构拆分：IM 平台层 → platform/telegram/

## 关键发现

AI Agent 层（AgentLoop/Memory/MCP/Tools/Natural/Guardrails/Tracing）对 IM 层（AgentAPI/types/configs/helpers）**零代码依赖**。拆分非常干净，不需要处理循环引用。

## 拆分后的目录结构

```
zapry-agents-sdk-go/
│
├── go.mod                      # module github.com/cyberFlowTech/zapry-agents-sdk-go
│
│  ── AI Agent 框架层（package agentsdk，根目录）──
├── agent_loop.go               # AgentLoop + RunContext
├── tools.go                    # ToolRegistry + ToolDef
├── tools_openai.go             # OpenAI Adapter
├── memory_*.go (7 files)       # Memory 三层模型
├── mcp_*.go (5 files)          # MCP Client
├── natural_*.go + prompt_*.go  # Natural Conversation (8 files)
├── guardrails.go               # Guardrails
├── tracing.go                  # Tracing
├── feedback.go                 # Feedback Detector
├── proactive.go                # Proactive Scheduler
├── agent_card.go               # Agent Card
├── agent_engine.go             # Handoff Engine
├── agent_handoff.go            # Handoff types
├── agent_policy.go             # Handoff Policy
├── agent_registry.go           # Agent Registry
│
│  ── IM 平台通讯层（package telegram）──
├── platform/
│   └── telegram/
│       ├── api.go              # AgentAPI（原 agent.go）
│       ├── types.go            # 原 types.go (3377 行)
│       ├── configs.go          # 原 configs.go (2570 行)
│       ├── helpers.go          # 原 helpers.go
│       ├── params.go           # 原 params.go
│       ├── compat.go           # 原 compat.go（Zapry 兼容层）
│       ├── passport.go         # 原 passport.go
│       ├── router.go           # 原 router.go
│       ├── agent.go            # ZapryAgent（原 zapry_agent.go）
│       ├── config.go           # AgentConfig（原 agent_config.go）
│       ├── middleware.go        # 原 middleware.go
│       ├── log.go              # 原 log.go
│       └── logger_util.go      # 原 logger_util.go
│
├── examples/                   # 更新 import 路径
│
│  ── 测试 ──
├── *_test.go                   # Agent 框架测试（留在根目录）
└── platform/telegram/*_test.go # IM 层测试（移到子包）
```

## 执行步骤

### Step 1: 创建 platform/telegram/ 目录，移动文件

移动 13 个文件：


| 原位置               | 新位置                                |
| ----------------- | ---------------------------------- |
| `agent.go`        | `platform/telegram/api.go`         |
| `types.go`        | `platform/telegram/types.go`       |
| `configs.go`      | `platform/telegram/configs.go`     |
| `helpers.go`      | `platform/telegram/helpers.go`     |
| `params.go`       | `platform/telegram/params.go`      |
| `compat.go`       | `platform/telegram/compat.go`      |
| `passport.go`     | `platform/telegram/passport.go`    |
| `router.go`       | `platform/telegram/router.go`      |
| `zapry_agent.go`  | `platform/telegram/agent.go`       |
| `agent_config.go` | `platform/telegram/config.go`      |
| `middleware.go`   | `platform/telegram/middleware.go`  |
| `log.go`          | `platform/telegram/log.go`         |
| `logger_util.go`  | `platform/telegram/logger_util.go` |


移动 5 个测试文件：


| 原位置                  | 新位置                                    |
| -------------------- | -------------------------------------- |
| `agent_test.go`      | `platform/telegram/api_test.go`        |
| `helpers_test.go`    | `platform/telegram/helpers_test.go`    |
| `middleware_test.go` | `platform/telegram/middleware_test.go` |
| `params_test.go`     | `platform/telegram/params_test.go`     |
| `types_test.go`      | `platform/telegram/types_test.go`      |


### Step 2: 修改 package 声明

所有移动到 `platform/telegram/` 的文件：

```go
// 从
package agentsdk
// 改为
package telegram
```

### Step 3: 更新 examples/ 的 import

```go
// 从
import agentsdk "github.com/cyberFlowTech/zapry-agents-sdk-go"
// 改为
import "github.com/cyberFlowTech/zapry-agents-sdk-go/platform/telegram"
```

### Step 4: 根目录提供 re-export（可选的兼容层）

如果你以后想让用户仍然能从根包访问平台类型，可以在根目录加一个 `platform_compat.go`：

```go
package agentsdk

import "github.com/cyberFlowTech/zapry-agents-sdk-go/platform/telegram"

// Re-export for backwards compatibility (optional)
type AgentAPI = telegram.AgentAPI
type ZapryAgent = telegram.ZapryAgent
// ...
```

但既然只有你一个用户，**不需要做兼容层**，直接改 import 即可。

### Step 5: 更新 README

Project Structure 部分按新结构更新。

## 不动的文件（34 个，全部留在根目录）

所有 AI Agent 框架文件不动：`agent_loop.go`、`tools*.go`、`memory_*.go`、`mcp_*.go`、`natural_*.go`、`guardrails.go`、`tracing.go`、`feedback.go`、`proactive.go`、`agent_card.go`、`agent_engine.go`、`agent_handoff.go`、`agent_policy.go`、`agent_registry.go` 及其测试。