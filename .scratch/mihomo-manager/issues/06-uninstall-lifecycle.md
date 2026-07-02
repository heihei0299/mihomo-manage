Status: completed

# 卸载生命周期

## Parent

`.scratch/mihomo-manager/PRD.md`

## What to build

实现 `Manager.Uninstall()` — 完整的 uninstall-lifecycle：

1. **stop** — 停止当前实例
2. **deregister** — 移除 systemd/launchd 服务文件并禁用服务
3. **cleanup** — 删除二进制文件和配置目录。用户通过 `--keep-backup` 标记选择是否保留备份。保留备份时，将 `/opt/mihomo/` 整体移动到用户指定的备份路径（默认 `/opt/mihomo.bak.<timestamp>`）而非直接删除。

### CLI

```
mihomo-manager uninstall              # 删除所有内容
mihomo-manager uninstall --keep-backup  # 保留备份
```

### TUI

卸载前弹出确认对话框，清晰说明后果（删除/保留备份）。提供 "--keep-backup" 选项切换。卸载后展示结果信息，自动退出 TUI 或回到"未安装"状态。

## Acceptance criteria

- [ ] `uninstall` 走通完整卸载流程
- [ ] `--keep-backup` 时配置和二进制被移动到备份路径而非删除
- [ ] 无 `--keep-backup` 时配置和二进制被彻底删除
- [ ] 服务注册被正确移除
- [ ] 卸载过程中显示进度
- [ ] 卸载后 `status` 返回 "not installed"
- [ ] 对一个已经不存在的实例执行 `uninstall` 返回明确错误
- [ ] Manager 模块测试覆盖卸载的各种场景

## Blocked by

- `.scratch/mihomo-manager/issues/02-install-lifecycle.md`
