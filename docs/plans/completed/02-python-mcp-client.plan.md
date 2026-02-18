---
name: Python SDK MCP Client
overview: 为 zapry-agents-sdk-python 实现 MCP Client，与 Go SDK v4 方案完全对齐。Python 天然 async，使用 asyncio.create_subprocess_exec（Stdio）和标准库 urllib/aiohttp-free 的 asyncio HTTP（HTTP），不引入新依赖。ToolDef 新增 raw_json_schema，MCPManager async API。
todos:
  - id: py-m0-rawschema
    content: "M0: registry.py ToolDef 新增 raw_json_schema + to_json_schema 优先用 + 2 测试"
    status: completed
  - id: py-m1-config
    content: "M1: mcp/config.py — MCPServerConfig + is_tool_allowed（fnmatch）"
    status: completed
  - id: py-m1-transport
    content: "M1: mcp/transport.py — MCPTransport Protocol + HTTPTransport + StdioTransport + InProcessTransport"
    status: completed
  - id: py-m1-protocol
    content: "M1: mcp/protocol.py — JSON-RPC 2.0 + MCPClient + MCPError"
    status: completed
  - id: py-m1-converter
    content: "M1: mcp/converter.py — convert_mcp_tools + mcp_result_to_text"
    status: completed
  - id: py-m1-manager
    content: "M1: mcp/manager.py — MCPManager"
    status: completed
  - id: py-m1-test
    content: "M1: tests/test_mcp.py — 全量测试 ~36 用例"
    status: completed
  - id: py-m2-exports
    content: "M2: __init__.py 导出 + README MCP 章节"
    status: completed
isProject: false
---

# Python SDK MCP Client 实现方案

与 Go SDK v4 完全对齐的 Python 实现。

## 已确认的 Python SDK 架构

- `zapry_agents_sdk/tools/registry.py` — `ToolDef`、`ToolParam`、`ToolContext`、`ToolRegistry`、`@tool` 装饰器
- `zapry_agents_sdk/agent/loop.py` — `AgentLoop`（async）、`AgentResult`、`AgentHooks`
- `zapry_agents_sdk/__init__.py` — 统一导出
- `pyproject.toml` — 依赖：`python-telegram-bot` + `python-dotenv`，无 aiohttp
- 测试用 `pytest` + `pytest-asyncio`

## 与 Go SDK 对齐的核心决策

- `ToolDef` 新增 `raw_json_schema: Optional[Dict]`，`to_json_schema()` 优先使用
- `ToolContext` 新增可选字段（Python 已经有 async 上下文，不需要额外 ctx 字段）
- Transport: `Call` 语义（`async def call(payload) -> bytes`）
- HTTPTransport: `urllib.request` 在 `asyncio.to_thread` 中运行（零依赖）
- StdioTransport: `asyncio.create_subprocess_exec` + 长期 reader task
- 通配符过滤匹配原始 MCP tool name，用 `fnmatch.fnmatch`
- InjectTools 幂等，RemoveTools 精确
- 全链路 async

## 新增文件

- `zapry_agents_sdk/mcp/__init__.py` — 模块入口 + 导出
- `zapry_agents_sdk/mcp/config.py` — `MCPServerConfig`、`MCPManagerConfig`、`is_tool_allowed`
- `zapry_agents_sdk/mcp/transport.py` — `MCPTransport`（Protocol）、`HTTPTransport`、`StdioTransport`、`InProcessTransport`
- `zapry_agents_sdk/mcp/protocol.py` — JSON-RPC 2.0、`MCPClient`、`MCPError`
- `zapry_agents_sdk/mcp/converter.py` — `convert_mcp_tools`、`mcp_result_to_text`
- `zapry_agents_sdk/mcp/manager.py` — `MCPManager`
- `tests/test_mcp.py` — 全量测试

## 修改文件

- `zapry_agents_sdk/tools/registry.py` — `ToolDef` 新增 `raw_json_schema` + `to_json_schema` 优先用
- `zapry_agents_sdk/__init__.py` — 新增 `MCPManager`、`MCPServerConfig` 导出
- `README.md` — MCP Client 章节

## M0: ToolDef.raw_json_schema (registry.py)

```python
@dataclass
class ToolDef:
    name: str
    description: str
    parameters: List[ToolParam] = field(default_factory=list)
    handler: Optional[Callable] = None
    is_async: bool = True
    raw_json_schema: Optional[Dict[str, Any]] = None  # 新增

    def to_json_schema(self) -> Dict[str, Any]:
        if self.raw_json_schema is not None:
            return {
                "name": self.name,
                "description": self.description,
                "parameters": self.raw_json_schema,
            }
        # ... 原有 ToolParam 构建逻辑不变 ...
```

## M1: mcp/config.py

```python
@dataclass
class MCPServerConfig:
    name: str
    transport: str = "stdio"  # "stdio" | "http"
    command: str = ""
    args: List[str] = field(default_factory=list)
    env: Dict[str, str] = field(default_factory=dict)
    url: str = ""
    headers: Dict[str, str] = field(default_factory=dict)
    timeout: int = 30
    max_retries: int = 3
    allowed_tools: List[str] = field(default_factory=list)
    blocked_tools: List[str] = field(default_factory=list)
    max_tools: int = 0

def is_tool_allowed(name: str, config: MCPServerConfig) -> bool:
    # fnmatch.fnmatch 通配符
```

## M1: mcp/transport.py

```python
class MCPTransport(Protocol):
    async def start(self) -> None: ...
    async def call(self, payload: bytes) -> bytes: ...
    async def close(self) -> None: ...

class HTTPTransport:
    # urllib.request in asyncio.to_thread (零依赖)
    async def call(self, payload: bytes) -> bytes:
        return await asyncio.to_thread(self._sync_call, payload)

class StdioTransport:
    # asyncio.create_subprocess_exec
    # 长期 reader task: process.stdout.readline() -> asyncio.Queue
    # stderr 消费 task: log only

class InProcessTransport:
    # handler: Callable[[bytes], bytes] (测试用)
```

## M1: mcp/protocol.py — MCPClient

```python
class MCPClient:
    async def initialize(self) -> MCPInitResult
    async def list_tools(self) -> List[MCPToolDef]  # 兼容 {tools:[...]} 和 [...]
    async def call_tool(self, name, args) -> MCPToolResult
    async def close(self)
```

## M1: mcp/converter.py

```python
def convert_mcp_tools(
    server_name: str,
    mcp_tools: List[MCPToolDef],
    call_fn: Callable,
    config: Optional[MCPServerConfig],
) -> List[ToolDef]:
    # 1. mcp.{server}.{tool} 命名
    # 2. raw_json_schema = inputSchema
    # 3. handler 闭包 async
    # 4. fnmatch 通配符过滤
    # 5. max_tools 截断
```

## M1: mcp/manager.py — MCPManager

```python
class MCPManager:
    async def add_server(self, config: MCPServerConfig) -> None
    async def add_server_with_transport(self, config, transport) -> None
    async def remove_server(self, name: str) -> None
    def inject_tools(self, registry: ToolRegistry) -> None  # 幂等
    def remove_tools(self, registry: ToolRegistry) -> None   # 精确
    async def call_tool(self, sdk_name, args) -> Any         # 含重试
    async def refresh_tools(self, *servers) -> None
    async def disconnect_all(self) -> None
    def list_tools(self, *servers) -> List[ToolDef]
    def server_names(self) -> List[str]
```

## 测试矩阵 (~36 用例)

与 Go SDK 完全对齐：

- 协议层 8 个（JSON-RPC 编解码、Initialize、ListTools 双格式、CallTool）
- 转换层 8 个（raw schema、extractParams、required、过滤、MaxTools）
- Manager 11 个（Add/Remove/Inject 幂等/RemoveTools 精确/E2E/多 server/刷新/重试）
- 集成测试 7 个（AgentLoop 选 MCP 工具、混合工具、超时、取消、通配符）
- Stdio 测试用 InProcessTransport 模拟（不依赖外部进程）

