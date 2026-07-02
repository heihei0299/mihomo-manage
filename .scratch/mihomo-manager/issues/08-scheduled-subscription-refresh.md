Status: ready-for-agent

# 定时订阅刷新

## Parent

`.scratch/mihomo-manager/PRD.md`

## What to build

实现定时自动刷新订阅配置的功能。

CLI 接口：

```
mihomo-manager subscription schedule --interval 24h    # 每天刷新一次
mihomo-manager subscription schedule --off             # 关闭定时刷新
mihomo-manager subscription status                     # 查看订阅状态（上次刷新时间、下次刷新时间）
```

后台机制：

- Manager 启动一个 goroutine/ticker 按配置间隔循环执行 `UpdateConfig`
- 调度配置持久化到 `/opt/mihomo-manager/state/schedule.txt`（格式如 `interval=24h`）
- TUI 启动时自动读取调度配置，如果启用则启动后台 ticker
- 多次刷新间隔至少 1 小时，防止误配置导致频繁请求

## Acceptance criteria

- [ ] `subscription schedule --interval 24h` 启动定时刷新并持久化配置
- [ ] `subscription schedule --off` 停止定时刷新
- [ ] 定时刷新周期到达时自动执行 UpdateConfig
- [ ] 刷新间隔下限 1 小时，小于此值返回错误
- [ ] TUI 启动时自动恢复上次的调度配置
- [ ] Manager 模块有测试覆盖调度逻辑

## Blocked by

None - can start immediately
