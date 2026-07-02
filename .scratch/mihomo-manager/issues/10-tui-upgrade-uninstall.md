Status: ready-for-agent

# TUI 升级版本选择 + 卸载流程

## Parent

`.scratch/mihomo-manager/PRD.md`

## What to build

补齐 TUI 中两个缺失的功能块。

### 升级版本选择

当前 TUI 按 `5` 直接升级到 "latest"（由 `resolveVersion()` 解析为最新 tag）。改为按 `5` 后显示最近 5 个版本的列表，用户通过方向键选择版本，回车确认升级。版本列表通过 `ListVersions` 获取。

### 卸载流程

TUI 当前没有任何卸载入口。新增卸载流程：
1. 按 `u` 触发卸载
2. 弹出确认对话框："确定卸载 mihomo？此操作将停止服务并删除文件"
3. 提供 `--keep-backup` 选项切换（默认否）
4. 确认后执行 `Uninstall(keepBackup)`，显示分阶段进度
5. 完成后回到仪表盘（状态 = not installed）

## Acceptance criteria

- [ ] TUI 升级显示版本列表供用户选择
- [ ] TUI 卸载有确认对话框
- [ ] 卸载可选 keep-backup，默认不保留
- [ ] 卸载过程显示分阶段进度
- [ ] 升级/卸载完成后仪表盘状态正确更新
- [ ] 测试覆盖 TUI 交互逻辑（通过 mock Manager）

## Blocked by

None - can start immediately
