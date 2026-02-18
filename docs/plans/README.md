# SDK Plans & Roadmap

## Completed

| # | Plan | Description | Commit |
|---|------|-------------|--------|
| 01 | [MCP Client](completed/01-mcp-client.plan.md) | MCP Client support (Stdio/HTTP transport) for Go SDK | `71bee97` |
| 02 | [Python MCP Client](completed/02-python-mcp-client.plan.md) | MCP Client for Python SDK (mirrors Go) | `5a2668d` |
| 03 | [AgentLoop Context/Cancel](completed/03-agentloop-context-cancel.plan.md) | RunContext with cancel/timeout for Go + Python | `67abd69` / `39fa579` |
| 04 | [Natural Conversation v2.1](completed/04-natural-conversation-v2.plan.md) | StateTracker, EmotionDetector, StyleController, Opener, Compressor | `df3adda` / `99561d4` |
| 05 | [Split IM Platform Layer](completed/05-split-im-platform-layer.plan.md) | channel/telegram/ + channel/zapry/ separation | `8558bc9` / `fc5ee02` |

## Pending

| # | Plan | Description | Priority |
|---|------|-------------|----------|
| 01 | [Stage 1: Agent Individual](pending/01-stage1-agent-individual.plan.md) | Loop Detection + AgentProfile + Persona Bridge | Next |

## Archived (shelved, kept for reference)

| Plan | Reason |
|------|--------|
| [Human-like Reply SDK](archived/human-like-reply-sdk.plan.md) | Replaced by Natural Conversation v2.1 |
| [Invisible agentd Sidecar](archived/invisible-agentd-sidecar.plan.md) | Deferred — ROI not justified for 2 SDKs |
| [Go-First SDK Sidecar](archived/go-first-sdk-sidecar.plan.md) | Deferred — same reason |
| [Zero-Config Persona Sidecar](archived/zero-config-persona-sidecar.plan.md) | Deferred — Persona integration via interface instead |
