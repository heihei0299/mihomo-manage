Status: ready-for-agent

# 06 — 卸载 + TUN 网卡

## Parent

`.scratch/acceptance-tests/PRD.md`

## What to build

卸载（完整删除）、卸载保留备份、TUN 网卡确认的验收测试。

注意：卸载测试会破坏环境，应作为测试序列的最后一项执行。

### TestAcceptanceUninstall
- `sudo mihomo-manager uninstall` → 退出码 0
- 输出包含 `[stop]`、`[unregister]`、`[clean]` 阶段
- 验证：`/opt/mihomo/bin/mihomo` 不存在
- 验证：`/opt/mihomo/etc/` 不存在
- 验证：`/opt/mihomo-manager/` 不存在
- 验证：`systemctl list-units | grep mihomo` 无匹配
- 验证：`mihomo-manager status` → `mihomo: not installed`，退出码 2
- 未安装时 uninstall → 退出码非 0

### TestAcceptanceUninstallKeepBackup
- 重新安装 mihomo，确保有订阅数据
- `sudo mihomo-manager uninstall --keep-backup` → 退出码 0
- 验证：`/opt/mihomo/bin/mihomo` 不存在
- 验证：`/opt/mihomo/etc/config.yaml` 或 `config.yaml.bak.*` 存在
- 验证：`/opt/mihomo-manager/backups/` 存在且非空
- 验证：systemd 服务已清理

### TestAcceptanceTunInterface
- 确认配置中有 `tun:` 段（否则跳过）
- `ip link show meta` → 输出包含 `UP` 和 `LOWER_UP`
- mihomo 停止后 meta 网卡可能消失（属正常行为）

## Acceptance criteria

- [ ] 3 个测试全部通过
- [ ] 卸载测试破坏环境后，后续通过 `mihomo-manager install` 可恢复
- [ ] TUN 测试在配置无 TUN 时跳过（不失败）

## Blocked by

- #02 — 实例状态 + 启停控制
