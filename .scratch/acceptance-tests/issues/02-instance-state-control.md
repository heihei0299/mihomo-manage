Status: completed

# 02 — 实例状态 + 启停控制

## Parent

`.scratch/acceptance-tests/PRD.md`

## What to build

实例状态查看、systemd 服务确认、停止、启动、重载、重启的验收测试。

这些测试共享安装前置（#01），测试前确保 mihomo 已安装且运行中。

### TestAcceptanceSystemdService
- `systemctl is-active mihomo` → `active`
- `systemctl is-enabled mihomo` → `enabled`
- `systemctl status mihomo --no-pager` 包含 `active (running)`

### TestAcceptanceStatus
- 运行中时：输出 `mihomo: running  (version: vX.Y.Z)`，退出码 0
- 停止后：输出 `mihomo: stopped  (version: vX.Y.Z)`，退出码非 0
- 卸载后：输出 `mihomo: not installed`，退出码非 0

### TestAcceptanceStop
- 停止后 `systemctl is-active mihomo` → `inactive`
- `pgrep -x mihomo` 退出码非 0
- `mihomo-manager status` 退出码非 0

### TestAcceptanceStart
- 从停止状态启动后 `systemctl is-active mihomo` → `active`
- `pgrep -x mihomo` 退出码 0
- `mihomo-manager status` 退出码 0

### TestAcceptanceReload
- 重载后 PID 不变（记录前后 PID 对比）
- 重载后服务状态 `active`

### TestAcceptanceRestart
- 重启后 PID 改变
- 重启后服务状态 `active`

## Acceptance criteria

- [ ] 6 个测试全部通过
- [ ] 每个测试独立可运行（`-run TestAcceptanceStart`）
- [ ] 每个测试在结束后 mihomo 回到运行中状态（不影响后续测试）

## Blocked by

- #01 — 基础设施 + 安装验证
