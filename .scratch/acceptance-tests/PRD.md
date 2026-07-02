Status: ready-for-agent

# 自动化验收测试

## Problem Statement

mihomo-manager 目前有 60+ 单元测试覆盖 Manager 接口的业务逻辑，但缺少端到端验收测试。每次修改核心逻辑后，开发者需要手动执行 22 个验收测试用例（见 `docs/acceptance-tests.md`），耗时且容易遗漏。典型回归场景：修改了 `isActive` 的退出码处理逻辑，手动未测到导致 `stop`/TUI 崩溃（见之前的 bug 修复）。

需要一套可自动执行的验收测试，在真实 systemd/launchd 环境中验证工具的完整行为。

## Solution

一套基于 Go `testing` 的验收测试套件，seam 为编译后的二进制 + shell 命令。测试脚本编译当前源码、以子进程方式运行 `mihomo-manager`，断言 stdout/stderr/退出码，辅以 `systemctl`、`ip link`、`journalctl`、`ls` 等 shell 命令验证系统状态。

测试在 Linux (systemd) 上运行，macOS (launchd) 的支持作为第二阶段目标。

## User Stories

1. As a developer, I want `TestAcceptanceInstall` to automatically verify that installing mihomo outputs 5 expected phases and creates all required files, so that I can catch installation regressions without manual testing.
2. As a developer, I want `TestAcceptanceStatus` to verify all three status states (running/stopped/not installed) with correct exit codes and output text, so that I can catch status formatting regressions.
3. As a developer, I want `TestAcceptanceStop` to verify that stopping changes the systemd state to inactive, kills the process, and updates `mihomo-manager status` output, so that I can catch stop-path regressions.
4. As a developer, I want `TestAcceptanceStart` to verify that starting transitions the instance from stopped to active, so that I can catch start-path regressions.
5. As a developer, I want `TestAcceptanceReload` to verify that reload keeps the PID unchanged while applying new config, so that I can catch hot-reload regressions.
6. As a developer, I want `TestAcceptanceRestart` to verify that restart produces a new PID and the service returns to active, so that I can catch restart-path regressions.
7. As a developer, I want `TestAcceptanceSubscriptionSet` to verify that setting a subscription URL writes the correct files, so that I can catch subscription-path regressions.
8. As a developer, I want `TestAcceptanceSubscriptionUpdate` to verify that updating a subscription fetches remote data, creates a backup, and reloads config without service interruption, so that I can catch update-path regressions.
9. As a developer, I want `TestAcceptanceConfigPreview` to verify that the preview output contains expected YAML keys, so that I can catch config-pipeline regressions.
10. As a developer, I want `TestAcceptanceUpgrade` to verify that upgrading to a new version updates the binary and creates a backup, so that I can catch upgrade-path regressions.
11. As a developer, I want `TestAcceptanceUpgradeRollback` to verify that a failed upgrade automatically restores the old binary and restarts the service, so that I can catch rollback regressions.
12. As a developer, I want `TestAcceptanceSchedule` to verify setting, querying, and stopping the auto-refresh schedule, so that I can catch schedule-path regressions.
13. As a developer, I want `TestAcceptanceLogs` to verify that `mihomo-manager logs` outputs journalctl-formatted lines and respects `--tail` / `--follow` flags, so that I can catch log-viewing regressions.
14. As a developer, I want `TestAcceptanceQuietMode` to verify that `--quiet` suppresses stdout but not stderr, and exit codes are correct, so that I can catch quiet-mode regressions.
15. As a developer, I want `TestAcceptanceUninstall` to verify that uninstalling removes all files and systemd registration, so that I can catch uninstall-path regressions.
16. As a developer, I want `TestAcceptanceUninstallKeepBackup` to verify that `--keep-backup` preserves config and backup directories, so that I can catch backup-preservation regressions.
17. As a developer, I want `TestAcceptanceTunInterface` to verify that when TUN is enabled in config, the `meta` network interface exists with UP status, so that I can catch TUN-mode regressions.
18. As a developer, I want `TestAcceptanceSystemdService` to verify that the mihomo systemd service is active and enabled at boot, so that I can catch service-registration regressions.

## Implementation Decisions

### Test seam

单一端到端 seam：编译后的二进制 + shell 命令。每个测试函数：

1. `go build` 编译当前源码为临时路径
2. 用 `os/exec` 运行 `mihomo-manager <command>`，捕获 stdout/stderr/退出码
3. 用 `os/exec` 运行辅助 shell 命令（`systemctl`、`ip link`、`journalctl`、`ls`）验证系统状态
4. 清理：卸载 mihomo 恢复环境

### 测试文件组织

新的测试包 `acceptance`，位于项目根目录 `acceptance/acceptance_test.go`（或 `test/acceptance_test.go`）。使用 build tag `//go:build acceptance` 隔离，不纳入 `go test ./...` 的默认执行。

运行方式：
```bash
go test -tags=acceptance ./acceptance/ -count=1 -v
```

### 前置条件

测试脚本自动执行以下校验，不满足时跳过（t.Skip）：

1. `sudo -n true`（无密码 sudo）
2. `systemctl --version`（Linux systemd 环境）
3. 可访问 `github.com`
4. mihomo 未安装（或测试前自动卸载）

每个测试独立运行，顺序：install → status → start/stop → config → subscription → upgrade → uninstall。

允许单个测试独立执行（t.SkipUnless 模式），但推荐全序列。

### Mock 策略

验收测试不使用 mock——它们就是用来验证真实集成的。但对于 AT-16（升级失败回滚），测试通过替换二进制为一个假脚本来模拟启动失败。

### 失败报告

每次失败输出：
- 运行的完整命令（含参数）
- 实际 stdout
- 实际 stderr
- 实际退出码
- 期望的文本/退出码差异

### 实现顺序

1. 基础设施：编译 + 运行辅助函数、测试框架、build tag
2. AT-01 安装（最核心，作为其他测试的前置）
3. AT-02 查看状态
4. AT-04 systemd 服务
5. AT-05/06 停止/启动
6. AT-07/08 重载/重启
7. AT-09/10/11 订阅设置/更新/预览
8. AT-12/13 编辑模板/规则（调用 $EDITOR，交互式——需要特殊处理）
9. AT-14 版本列表
10. AT-15/16 升级/回滚
11. AT-17 定时刷新
12. AT-18 日志
13. AT-20 --quiet 模式
14. AT-21/22 卸载/保留备份卸载
15. AT-03 TUN 网卡（依赖 mihomo 配置文件含 tun 段）

## Testing Decisions

### 什么构成好的验收测试

- 测试真实行为而非实现：断言 stdout 文本、退出码、系统状态，不关心 Manager 内部方法调用
- 幂等且可恢复：每个测试清理自身副作用，测试序列可重复执行
- 优先全序列执行（install → use → uninstall），但单个测试也可独立运行
- 真实网络请求（订阅更新、版本列表）不 mock——但使用已知可访问的 URL 而非依赖第三方服务

### 哪些模块被测

- `acceptance/` 包下的验收测试
- `go test -tags=acceptance` 独立运行，不干扰 `go test ./...`

### 先例

现有 60+ 单元测试（`manager_test.go`、`lifecycle_test.go`、`servicemanager_test.go`）提供了测试惯例参考：表驱动测试、mockSystem、commandRecorder。验收测试遵循相同的 Go testing 惯例，但 seam 从 Manager 接口变为二进制文件。

## Out of Scope

- TUI 验收测试（AT-19）。Bubble Tea TUI 的交互测试需要 `tview`/`teatest` 或屏幕截图对比，复杂度高且收益低。维持手动验收。
- macOS launchd 验收测试。第一个版本专注于 Linux systemd，macOS 支持为后续。
- 跨架构测试（arm64、riscv64 等）。仅在 amd64 上运行。
- 性能测试。不测量安装耗时、下载速度等。

## Further Notes

- 验收测试需要 sudo 权限：在 CI 中通过 `sudo -E env "PATH=$PATH" go test -tags=acceptance` 执行。
- Docker 化：可考虑提供 Dockerfile 基于 ubuntu:22.04 创建干净的测试环境，安装 mihomo-manager 依赖（systemd、iproute2 等），在容器内执行验收测试。
- 与 CI 集成：验收测试不阻塞每次 push，但应在 release 分支上自动触发。
