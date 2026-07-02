Status: ready-for-agent

## Parent

Architecture deepening — [architecture review](/tmp/architecture-review-1783028521.html)

## What to build

将调度器从 `manager` 结构体提取为独立的 `Scheduler` 模块，封装 goroutine 生命周期、持久化状态和错误处理。

定义接口：

```go
type Scheduler interface {
    Start(ctx context.Context, interval time.Duration, task func(context.Context)) error
    Stop(ctx context.Context) error
    Status(ctx context.Context) (interval time.Duration, active bool, err error)
}
```

实现要点：
- 内部通过 `FileSystem` seam 管理 `schedule.txt`（持久化 interval 和 `"off"`）
- goroutine 中任务执行失败时通过回调上报，继续运行（不再静默吞掉）
- `manager` 结构体移除 `mu`、`ticker`、`stopCh` 三个字段
- `manager.SetSchedule` / `StopSchedule` / `ScheduleStatus` 委托给 scheduler

调度器使用 `ConfigPipeline.Apply` 作为回调（见 Issue 03）。

详见 ADR-0005。

## Acceptance criteria

- [ ] `Scheduler` 接口和实现存在，独立包 `internal/scheduler` 或同一包内
- [ ] `Start()` 校验 interval ≥ 1h 后启动 goroutine，ticker 按 interval 触发 task
- [ ] `Stop()` 停止 ticker 关闭 stopCh，写入 `"off"` 到 schedule 文件
- [ ] `Status()` 从文件读取 interval 并返回是否活跃
- [ ] task 执行失败时通过回调上报，ticker 继续运行
- [ ] `manager` 结构体移除 `mu`、`ticker`、`stopCh` 字段
- [ ] `manager.SetSchedule` / `StopSchedule` / `ScheduleStatus` 委托给 scheduler
- [ ] 所有测试通过
- [ ] schedule 文件格式兼容旧版本（向后兼容）

## Blocked by

None — but 强烈建议在 Issue 03（ConfigPipeline）之后做，因为 task 回调用 `pipeline.Apply`

## References

- ADR-0005: `docs/adr/0005-scheduler-module.md`
