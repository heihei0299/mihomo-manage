Status: ready-for-agent

# 实例运行控制

## Parent

`.scratch/mihomo-manager/PRD.md`

## What to build

实现 `Manager` 的四个 control-operation：`Start()`、`Stop()`、`Restart()`、`Reload()`。含状态机合法性校验。

### 状态机约束

```
stopped  ──start──▶  running
running  ──stop──▶   stopped
running  ──reload──▶ running  (reload in-place)
running  ──restart──▶ stopped → running
running  ──upgrade──▶ upgrading  (Slice 5 使用)
any      ──failed──▶ failed     (由底层进程异常进入)
```

非法操作返回明确错误（如 "cannot start: instance is already running"）。

### 底层实现

- Linux：通过 systemd（`systemctl start/stop/restart/reload mihomo`）
- macOS：通过 launchctl
- 无服务管理器环境（Docker 等）：直接进程管理（fork/exec + signal）

### CLI

```
mihomo-manager start
mihomo-manager stop
mihomo-manager restart
mihomo-manager reload
```

### TUI

在仪表盘上添加控制按钮（Start / Stop / Restart / Reload），根据当前状态动态显示可用按钮。例如 running 时显示 Stop 和 Restart，stopped 时显示 Start。

## Acceptance criteria

- [ ] 四个操作均通过 CLI 可用
- [ ] 状态机约束得到正确执行（非法操作返回错误，不改变状态）
- [ ] TUI 按钮根据当前状态动态启用/禁用
- [ ] 操作完成后 TUI 仪表盘状态及时更新
- [ ] `Reload()` 不重启进程（向 mihomo 发送 SIGHUP 或通过 API 重载）
- [ ] Manager 模块测试覆盖所有合法及非法状态转换

## Blocked by

- `.scratch/mihomo-manager/issues/02-install-lifecycle.md`
