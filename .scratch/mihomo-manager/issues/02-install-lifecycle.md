Status: completed

# 安装生命周期

## Parent

`.scratch/mihomo-manager/PRD.md`

## What to build

实现 `Manager.Install()` — 完整的 installation-lifecycle：

1. **fetch** — 从 GitHub Releases 下载指定版本的 mihomo 二进制
2. **deploy** — 将二进制放置到 `/opt/mihomo/bin/mihomo`，并设置可执行权限
3. **bootstrap** — 在 `/opt/mihomo/etc/` 下创建初始 config-template.yaml（含默认模板，包含 `{{subscription}}` 和 `{{routing_rules}}` 占位）及初始 config.yaml
4. **register** — 注册 systemd 服务（Linux）或 launchd 服务（macOS），生成对应的 service 文件
5. **start** — 启动实例，验证服务正常运行

CLI 命令 `mihomo-manager install [--version <tag>]`，默认安装最新版。

TUI 新增安装流程视图：展示每个阶段的进度，出错时显示错误信息，并提供重试选项。

### 进度通知接口（原型）

```go
type ProgressEvent struct {
    Phase   InstallationPhase // fetch | deploy | bootstrap | register | start
    Message string
    Error   error // nil 表示成功
}
```

`Install()` 通过回调或 channel 发射进度事件，CLI 写入 stdout，TUI 更新进度条。

## Acceptance criteria

- [ ] `Manager.Install()` 走通完整 5 阶段流程
- [ ] 二进制下载失败时清理临时文件并返回有意义错误
- [ ] 任何阶段失败后回滚已完成的步骤（如 deploy 后 register 失败则清除已部署的文件）
- [ ] CLI 安装过程显示分阶段进度
- [ ] TUI 安装视图展示进度条 + 当前阶段文字说明
- [ ] 安装完成后 `status` 返回 `running`
- [ ] 安装完成后 `/opt/mihomo/etc/config-template.yaml` 存在且包含占位
- [ ] 通过 mock filesystem + mock process manager 测试 manager 模块

## Blocked by

- `.scratch/mihomo-manager/issues/01-project-scaffold-and-status.md`
