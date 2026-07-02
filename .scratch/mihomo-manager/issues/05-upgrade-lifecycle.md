Status: completed

# 升级生命周期

## Parent

`.scratch/mihomo-manager/PRD.md`

## What to build

实现 `Manager.Upgrade()` — 完整的 upgrade-lifecycle：

1. **check** — 从 mihomo GitHub Releases API 获取可用版本列表
2. **select** — 展示最近 5 个版本，默认选中 latest。用户通过 CLI `--version` 指定或 TUI 选择
3. **fetch** — 下载所选版本的二进制到临时路径
4. **stop** — 停止当前实例
5. **replace** — 将旧二进制备份到 `/opt/mihomo/bin/mihomo.bak.<version>`，新二进制放入 `/opt/mihomo/bin/mihomo`
6. **start** — 启动新版本实例
7. **rollback-on-fail** — 如果启动失败（进程退出/健康检查超时），自动恢复备份二进制并重启旧版本。记录错误日志。

### CLI

```
mihomo-manager versions               # 列出可用版本
mihomo-manager upgrade                # 升级到 latest
mihomo-manager upgrade --version v1.18.0  # 升级到指定版本
```

### TUI

升级视图：版本列表（最近 5 个，标记当前版本和最新版本）→ 选择版本 → 进度展示（下载/停止/替换/启动）→ 成功或失败结果。

## Acceptance criteria

- [ ] `versions` 命令列出 GitHub Releases 上最近 5 个版本
- [ ] `upgrade` 走通完整 upgrade-lifecycle
- [ ] 旧二进制在替换前正确备份
- [ ] 新实例启动失败后自动回滚到备份版本
- [ ] 升级过程中实例状态切换为 `upgrading`（影响 TUI 仪表盘）
- [ ] 升级成功/失败后 TUI 仪表盘状态正确更新
- [ ] 网络错误（GitHub 不可达）时给出清晰提示，不破坏现有配置
- [ ] Manager 模块测试覆盖 upgrade 各种场景（成功、启动失败回滚、下载失败等）

## Blocked by

- `.scratch/mihomo-manager/issues/02-install-lifecycle.md`
