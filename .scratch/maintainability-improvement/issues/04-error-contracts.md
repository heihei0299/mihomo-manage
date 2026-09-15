# 04: 统一 CLI/TUI 错误契约

**What to build:** 让 CLI、TUI 和 main 根据 manager 已提供的稳定错误契约进行分支，而不是解析错误文本；用户仍获得基本兼容的错误提示，调用方不再受文案变化影响。

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] 已有的 branchable sentinel errors（包括 config update busy、subscription source 未配置、mihomo 未安装、adopt confirmation 和实例状态错误）通过 `errors.Is` 判断。
- [x] UnsupportedPlatformError、LegacyScheduleError 等 typed errors 通过 `errors.As` 判断。
- [x] CLI/TUI/main 不再新增或保留针对已知契约的错误字符串 equality、substring 或 prefix 分支。
- [x] 普通错误传播语义保持不变。
- [x] 用户可见错误文本保持基本兼容。
- [x] 未新增错误层级；新增的实例状态 sentinel 是原计划明确要求的稳定 ServiceControl 契约。
- [x] Handler/TUI 相关测试覆盖错误分支选择，并证明分支不依赖错误字符串。

## 实施总结

- ServiceControl 对未安装、已运行、未运行状态返回稳定 sentinel errors，错误文本保持不变。
- adopt 与 schedule 的 Handler/TUI 测试改为验证 wrapped sentinel/typed errors 仍能正确分支。
- 扫描确认 main、CLI、TUI 生产代码不存在已知错误的字符串解析，因此未引入无语义的分类输出分支。
- 验证：`go test ./internal/manager`、`go test ./internal/cli`、`go test .` 均通过。
- Review：Spec review 通过；Standards 仅有与原计划冲突的 YAGNI 判断项，未发现硬性违规。
