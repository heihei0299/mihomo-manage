Status: ready-for-agent

# 04 — 升级 + 回滚

## Parent

`.scratch/acceptance-tests/PRD.md`

## What to build

版本列表、升级、升级失败回滚的验收测试。

### TestAcceptanceVersions
- 网络可达时：输出 5 行，每行 `v<数字>.<数字>.<数字>`
- 退出码 0
- 无网络时退出码非 0

### TestAcceptanceUpgrade
- 安装 mihomo 后记录当前版本
- `sudo mihomo-manager upgrade <同一版本>` — 可降级或同级重装（确保测试的幂等性）
- 输出包含 `[fetch]` → `[deploy]` → `[start]` 阶段
- 验证 `/opt/mihomo-manager/backups/mihomo.bak` 存在
- 验证 `systemctl is-active mihomo` → `active`

### TestAcceptanceUpgradeRollback
- 备份真实二进制，替换为 `#!/bin/sh\nexit 1`
- `sudo mihomo-manager upgrade v9.99.99` → 新版本启动失败
- 验证自动回滚：版本号恢复为旧版本
- 验证 `systemctl is-active mihomo` → `active`
- 恢复真实二进制

## Acceptance criteria

- [ ] 3 个测试全部通过
- [ ] 升级后恢复测试前的版本（幂等）
- [ ] 回滚测试结束后 mihomo 正常运行

## Blocked by

- #02 — 实例状态 + 启停控制
