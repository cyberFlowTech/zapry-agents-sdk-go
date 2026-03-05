# Dependency Policy

## 目标

在可维护性、稳定性与开发效率之间保持平衡，避免依赖无序增长。

## 策略

- 核心链路（`AgentLoop`、`ToolRegistry`、`MemorySession`、`Guardrail`）优先使用标准库。
- 外部依赖只在“明确增益”场景引入（例如 Redis 持久化存储）。
- 可选能力（如 Redis MemoryStore）应与核心链路解耦，不影响基础使用。
- 新增依赖必须满足：
  - 社区活跃、维护稳定；
  - 许可证兼容；
  - 有明确替代成本说明；
  - 具备测试覆盖与回归验证。

## 版本与升级

- 采用 SemVer（`MAJOR.MINOR.PATCH`）作为发布版本规范。
- 版本信息由 `version.go` 提供，并可通过 `-ldflags -X` 在构建时注入。
- 每次发布前更新 `docs/changelog.md`，记录行为变更与兼容性影响。
