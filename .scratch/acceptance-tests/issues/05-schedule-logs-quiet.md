Status: completed

# 05 — 定时 + 日志 + --quiet

## Parent

`.scratch/acceptance-tests/PRD.md`

## What to build

定时订阅刷新、日志查看、--quiet 模式的验收测试。

### TestAcceptanceSchedule
- 设置 6h 间隔：`mihomo-manager subscription schedule --interval 6h` → 退出码 0，输出 `schedule set to every 6h0m0s`
- 查看：`mihomo-manager subscription schedule` → 输出 `schedule: every 6h0m0s`
- 关闭：`mihomo-manager subscription schedule --off` → 退出码 0
- 查看关闭后：输出 `schedule: off`
- 拒绝过短间隔（30s）：退出码非 0，不生效

### TestAcceptanceLogs
- `mihomo-manager logs --tail=5` — 输出恰好 5 行，journalctl 格式
- `mihomo-manager logs --tail=100` — 输出 100 行（或全部）
- `mihomo-manager logs` — 输出默认 50 行
- 退出码均为 0

### TestAcceptanceQuietMode
- `mihomo-manager --quiet status`（运行中）— 无 stdout，退出码 0
- `mihomo-manager --quiet status`（卸载后）— 无 stdout，退出码 2
- `mihomo-manager -q status` — 短格式，行为同上
- 错误信息仍输出到 stderr

## Acceptance criteria

- [ ] 3 个测试全部通过
- [ ] 每个测试独立可运行

## Blocked by

- #02 — 实例状态 + 启停控制
