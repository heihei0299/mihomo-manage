Status: ready-for-agent

# 项目骨架 + 状态仪表盘

## Parent

`.scratch/mihomo-manager/PRD.md`

## What to build

使用 Go + Bubble Tea 搭建项目骨架，定义核心模块结构。建立 `Manager` 接口作为唯一的测试 seam，实现 CLI 入口 + TUI 入口的骨架。

核心交付：`status` 命令。检测系统上 mihomo 的安装状态和运行状态，输出结果。CLI 输出文本，TUI 渲染仪表盘。

### Manager 接口（原型）

```go
type Status struct {
    InstanceState InstanceState // stopped | running | upgrading | failed
    Installed     bool
    Version       string // mihomo 版本号，未安装时为空
}

type Manager interface {
    Status(ctx context.Context) (*Status, error)
    Install(ctx context.Context, version string) error    // 后续 slice
    Uninstall(ctx context.Context, keepBackup bool) error // 后续 slice
    Start(ctx context.Context) error     // 后续 slice
    Stop(ctx context.Context) error      // 后续 slice
    Restart(ctx context.Context) error   // 后续 slice
    Reload(ctx context.Context) error    // 后续 slice
    Upgrade(ctx context.Context, version string) error // 后续 slice
}
```

### CLI 入口

```
mihomo-manager status
```

### TUI 仪表盘

Bubble Tea 初始布局，展示状态信息。本次只渲染状态文本，不需要交互控件。

## Acceptance criteria

- [ ] `Manager` 接口定义包含所有领域操作的方法签名
- [ ] `Status()` 实现能检测 mihomo 二进制是否存在、实例是否正在运行
- [ ] CLI `mihomo-manager status` 输出易读的状态信息
- [ ] TUI 启动后展示仪表盘，显示当前状态
- [ ] Manager 模块有单元测试覆盖 `Status()` 的各种系统场景
- [ ] CLI 返回合适的退出码（0 = running, 1 = stopped/failed, 2 = not installed）

## Blocked by

None - can start immediately
