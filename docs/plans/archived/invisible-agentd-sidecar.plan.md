---
name: Invisible agentd Sidecar
overview: Go SDK 为唯一全功能实现，编译出 zapry-agentd 二进制。Python SDK 通过 platform-specific wheel 内嵌 agentd，首次 import 自动启动，开发者全程无感。agentd 通过 stdio MCP 暴露 Memory Session + Tracing + Persona 三个高层 API。
todos:
  - id: agentd-core
    content: "Phase 1: agentd_server.go + agentd_memory.go + cmd/agentd/main.go"
    status: pending
  - id: agentd-test
    content: "Phase 1: agentd_test.go ~10 个测试"
    status: pending
  - id: py-runtime
    content: "Phase 1: Python _runtime.py 自动启动 + RemoteMemorySession"
    status: pending
  - id: py-runtime-test
    content: "Phase 1: Python 测试 ~8 个"
    status: pending
  - id: agentd-tracing
    content: "Phase 2: agentd_tracing.go + trace.emit"
    status: pending
  - id: ci-wheels
    content: "Phase 2: CI/CD 6 平台交叉编译 + zapry-agentd PyPI wheel"
    status: pending
  - id: auto-download
    content: "Phase 2: 自动下载 fallback + SHA256 校验"
    status: pending
isProject: false
---

# 开发者无感 agentd Sidecar — 完整方案

## 核心体验目标

```python
# 开发者写的代码——完全不知道 agentd 的存在
from zapry_agents_sdk import AgentLoop, ToolRegistry, MemorySession, tool

session = MemorySession("my_agent", "user_123")  # 自动连 agentd
await session.load()
await session.add_message("user", "你好")

loop = AgentLoop(llm_fn=my_llm, tool_registry=registry,
                 system_prompt="You are helpful.")
result = await loop.run("你好", extra_context=session.format_for_prompt())
```

**开发者不需要**：安装额外包、启动额外进程、写任何配置、理解 MCP。

---

## 分 4 层实现

### Layer 1: agentd 二进制（Go，在 sdk-go 仓库）

agentd 是一个 Go 编译的二进制，stdio 模式运行，暴露 **5 个高层 MCP 工具**：


| 工具                        | 输入                                   | 输出                                 | 说明                 |
| ------------------------- | ------------------------------------ | ---------------------------------- | ------------------ |
| `memory.load`             | `{agent_id, user_id}`                | `{short_term, long_term, working}` | 一次返回全部记忆           |
| `memory.save`             | `{agent_id, user_id, role, content}` | `{ok}`                             | AddMessage + Trim  |
| `memory.update_long_term` | `{agent_id, user_id, updates}`       | `{merged}`                         | 增量更新长期记忆           |
| `memory.format`           | `{agent_id, user_id, template}`      | `{prompt}`                         | 返回可注入 LLM 的 prompt |
| `trace.emit`              | `{spans: [...]}`                     | `{accepted}`                       | 批量上报 spans         |


**为什么是高层 API 而不是底层 store 操作**：减少 RPC 次数（一次对话 2-3 次 vs 7-10 次），且全部业务逻辑在 agentd 内执行，Python 侧零逻辑。

**数据持久化**：agentd 内部用 Go SDK 已有的 `InMemoryMemoryStore`（开发模式）或基于文件的持久化 store（`~/.zapry/agentd/data/`）。

**不暴露的能力**（留在本地）：

- AgentLoop（用户的 LLM 函数和工具函数是本地的）
- ToolRegistry（同上）
- Guardrails（用户函数，无法远程化）
- Memory Extract（需要调用户的 LLM，留在本地触发）

新增文件（在 `zapry-agents-sdk-go` 仓库）：

```
cmd/agentd/main.go          # 二进制入口（读 stdin，分发，写 stdout）
agentd_server.go             # MCP Server 协议处理
agentd_memory.go             # memory.* handler（用 MemorySession 内部逻辑）
agentd_tracing.go            # trace.emit handler
agentd_test.go               # ~10 个测试
```

### Layer 2: 二进制分发（CI/CD）

**两种分发，优先 wheel：**

**方式 A: Platform-specific wheel（优先）**

发布一个 Python 包 `zapry-agentd`，里面只有一个预编译二进制 + 一个 Python 入口脚本：

```
zapry-agentd/
├── pyproject.toml
├── zapry_agentd/__init__.py      # 提供 get_binary_path() -> str
└── zapry_agentd/bin/
    └── zapry-agentd              # 预编译二进制（平台特定）
```

构建 6 个 wheel：

- `zapry_agentd-0.1.0-py3-none-macosx_11_0_arm64.whl`
- `zapry_agentd-0.1.0-py3-none-macosx_10_9_x86_64.whl`
- `zapry_agentd-0.1.0-py3-none-manylinux_2_17_x86_64.whl`
- `zapry_agentd-0.1.0-py3-none-manylinux_2_17_aarch64.whl`
- `zapry_agentd-0.1.0-py3-none-win_amd64.whl`
- `zapry_agentd-0.1.0-py3-none-win32.whl`

SDK 的 `pyproject.toml` 加依赖：`zapry-agentd >= 0.1.0`

这样 `pip install zapry-agents-sdk` 自动装 agentd 二进制。

**方式 B: 自动下载（fallback）**

如果 wheel 没装（比如用源码安装），SDK 首次 import 时：

1. 检测 `~/.zapry/bin/zapry-agentd` 是否存在
2. 不存在 → 从 GitHub Releases 下载对应平台的二进制
3. 校验 SHA256
4. 存到 `~/.zapry/bin/`，chmod +x

**方式 C: 都不可用（最终 fallback）**

SDK 内部回退到纯 Python 实现（现有的 `InMemoryStore`）。打一行日志：

```
[zapry] agentd not available. Using in-memory store (data will not persist).
```

### Layer 3: Python SDK 自动启动（对开发者完全隐藏）

在 Python SDK 中新增一个内部模块 `zapry_agents_sdk/_runtime.py`：

```python
# 全局单例，首次 import 时惰性初始化
_agentd: Optional[AgentdConnection] = None

def get_agentd() -> Optional[AgentdConnection]:
    """获取 agentd 连接（自动启动，失败返回 None）。"""
    global _agentd
    if _agentd is not None:
        return _agentd
    
    binary = _find_binary()  # wheel → ~/.zapry/bin → PATH → None
    if binary is None:
        _try_auto_download()
        binary = _find_binary()
    
    if binary is None:
        logger.info("agentd not available, using local fallback")
        return None
    
    _agentd = AgentdConnection(binary)  # StdioTransport + MCPClient
    atexit.register(_agentd.shutdown)   # 进程退出时自动清理
    return _agentd
```

### Layer 4: Python SDK API 透明切换

修改 `MemorySession` 的工厂方法，自动选择 agentd 或本地：

```python
class MemorySession:
    @classmethod
    def create(cls, agent_id, user_id, store=None):
        if store is not None:
            return cls(agent_id, user_id, store)  # 用户显式指定 store
        
        agentd = get_agentd()
        if agentd is not None:
            return RemoteMemorySession(agent_id, user_id, agentd)
        
        return cls(agent_id, user_id, InMemoryStore())  # fallback
```

`RemoteMemorySession` 实现相同的 API（load/add_message/format_for_prompt），但内部走 MCP 调用 agentd。

**关键**：`MemorySession` 的公开 API 完全不变。开发者不知道背后是本地还是远程。

---

## 开发者体验全流程

```bash
# 安装（agentd 二进制随 wheel 自动安装）
pip install zapry-agents-sdk

# 写代码（零 agentd 相关代码）
from zapry_agents_sdk import MemorySession, AgentLoop

session = MemorySession.create("my_bot", "user_123")
await session.load()
# ... 正常使用，数据自动持久化到 ~/.zapry/agentd/data/
```

**对比现在**：开发者现在用 `InMemoryStore`（重启丢数据）或 `SQLiteMemoryStore`（需要手动配路径）。有了 agentd 后，默认就是持久化的，而且用户无需做任何配置。

---

## 实施分期

### Phase 1（可交付，3-4 天）

聚焦 Memory，因为它是**开发者最容易感知到价值的**（数据持久化从"需要配置"变成"自动"）。

1. `agentd_server.go` — MCP stdio 入口
2. `agentd_memory.go` — memory.load/save/update_long_term/format
3. `cmd/agentd/main.go` — 二进制入口
4. `agentd_test.go` — 测试
5. Python `_runtime.py` — 自动启动 + 二进制查找
6. Python `RemoteMemorySession` — 透明切换
7. Python 测试

### Phase 2（+2 天）

1. `agentd_tracing.go` — trace.emit
2. CI/CD：GitHub Actions 交叉编译 6 平台
3. `zapry-agentd` PyPI wheel 包
4. 自动下载 fallback

### Phase 3（后续）

1. Persona 集成（等 persona-engine 依赖理清）
2. Node.js 薄客户端
3. MCP Proxy（agentd 代理外部 MCP Server）

---

## 测试策略

### agentd 测试（~10 个，用 InProcessTransport mock stdin/stdout）

- initialize 返回正确格式
- tools/list 返回 5 个工具
- memory.load 空 session → 返回空数据
- memory.save → memory.load 数据一致
- memory.update_long_term 增量合并
- memory.format 返回 prompt 字符串
- trace.emit 批量接受
- 无效工具名 → JSON-RPC error
- 无效参数 → 清晰错误信息
- 并发 save → 数据不丢

### Python 测试（~8 个，用 InProcessTransport mock agentd）

- RemoteMemorySession.load/save/format 端到端
- MemorySession.create 自动检测 agentd
- agentd 不可用 → fallback 到 InMemoryStore
- atexit 清理子进程
- 二进制查找优先级（wheel > ~/.zapry/bin > PATH）
- 与 AgentLoop 集成端到端

