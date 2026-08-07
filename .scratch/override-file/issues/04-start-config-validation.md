# 04 — start/restart 前配置校验

**What to build:** 新增配置校验能力（复用既有校验 seam）：CLI/TUI 的 start 与 restart 命令执行前先校验配置，失败则拒绝启动并输出校验错误。install/upgrade 流程内部的 start 不新增校验。

**Blocked by:** None — 可立即开始

**Status:** resolved

- [x] 校验方法测试：校验失败错误传播；成功时无副作用（`TestConfigManagerValidateConfigPropagatesFailure` / `SuccessNoSideEffects` / `NilValidatorPasses`）
- [x] start 命令校验失败 → 拒绝启动、输出错误、退出码非 0（`TestStartRejectedWhenValidationFails`：退出码 1、`control.Start` 未调用、stderr 输出校验错误）
- [x] restart 命令同样校验（`TestRestartRejectedWhenValidationFails`）
- [x] 校验通过时启动流程不受影响（`TestStartProceedsWhenValidationPasses` / `TestRestartProceedsWhenValidationPasses` / TUI 正向路径）

## 实施总结
- 提交：`e781105` — `feat(core): start/restart 执行前校验配置，失败拒绝启动`
- 提交：`e116b8c` — `fix(core): 补 Warn 回调并接入 mergeConfig 到 Preview，恢复基线编译与测试`（前置基线修复：dev 分支 327eb1c 遗留的 yaml.v3 依赖缺失、mergeConfig 未接入 pipeline、超前测试 Warn 字段缺失）
- 实现的 seams：
  - S1 `ConfigManager.ValidateConfig(ctx) error`（manager 包）：复用 `ConfigValidator` seam，委托 `configPipeline.Validate`（nil validator 时不校验）；失败错误传播、成功无副作用
  - S2 CLI `Handler.Start` 前校验：失败拒绝启动（`control.Start` 不被调用）、stderr 输出错误、返回 1
  - S3 CLI `Handler.Restart` 前校验：同 S2 restart 变体
  - S4 TUI `execActionCmd` actStart/actRestart 前校验：失败返回错误、`ctrl.Start/Restart` 不被调用
- 验收标准：4/4 全绿（见上方 checkbox）
- 测试结果：全绿（go test ./... 四包共 114 个测试通过）
- typecheck：go vet ./... 与 go build ./... 通过
- 文档对齐：无需更新（README 命令清单与实现一致；CONTEXT.md 已描述 ConfigValidator seam 与 start 前校验）
- 遗留 / 后续建议：无
