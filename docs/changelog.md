# Change Log

## Unreleased

- 新增 `version.go` 版本信息导出能力：`Version`、`GitCommit`、`BuildTime` 与 `GetVersionInfo()`。
- Guardrail 支持 context 透传的 V2 形态，便于接入异步/外部内容安全服务。
- AutoConversation 增加会话 TTL 与后台清理机制，补齐内存回收。
- 新增 `AutoConversationRuntime.Shutdown(ctx)` 与 `SDKRuntime.Shutdown(ctx)`，支持统一优雅关闭。
- 修复 `NaturalAgentLoop` 并发场景下的共享状态竞争问题。
- 增补运行时与生命周期相关测试用例。

## v5.4.0

- Remove all methods that return `(APIResponse, error)`.
  - Use the `Request` method instead.
  - For more information, see [Library Structure][library-structure].
- Remove all `New*Upload` and `New*Share` methods, replace with `New*`.
  - Use different [file types][files] to specify if upload or share.
- Rename `UploadFile` to `UploadFiles`, accept `[]RequestFile` instead of a
  single fieldname and file.
- Fix methods returning `APIResponse` and errors to always use pointers.
- Update user IDs to `int64` because of Bot API changes.
- Add missing Bot API features.

[library-structure]: ./getting-started/library-structure.md#methods
[files]: ./getting-started/files.md
