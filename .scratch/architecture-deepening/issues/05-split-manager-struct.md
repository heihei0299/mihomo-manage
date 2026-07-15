Status: ready-for-agent

## What to build

将 `manager` 结构体拆分为 4 个具体类型，各实现一个接口，各自持有仅需要的 seam。

- **`serviceController`** — 实现 `ServiceControl`，持有 `fs` + `cmd` + `svcMgr`
- **`lifecycleManager`** — 实现 `LifecycleManager`，持有 `fs` + `cmd` + `gh` + `svcMgr`
- **`configManager`** — 实现 `ConfigManager`，持有 `fs` + `gh` + `ConfigValidator`
- **`scheduler`** — 实现 `ScheduleManager`，持有 `fs` + task 回调（即模块 `internal/scheduler`）

现存的 4 个接口文件不动。`manager` 包中的具体类型可以放到新文件或现有文件中。

构造签名示例：
```go
func NewServiceController(fs FileSystem, cmd CommandRunner, svcMgr ServiceManager) ServiceControl
func NewLifecycleManager(fs FileSystem, cmd CommandRunner, gh GitHubReleases, svcMgr ServiceManager) LifecycleManager
func NewConfigManager(fs FileSystem, gh GitHubReleases, validate ConfigValidator) ConfigManager
// Scheduler 在 internal/scheduler 中已有构造函数
```

`manager` 包不再有中心化 `manager` 结构体。现有 `New()` 函数删除，各调用方直接构造所需的具体类型。

## Acceptance criteria

- [ ] `serviceController` 类型存在，只持有 `fs`、`cmd`、`svcMgr`
- [ ] `lifecycleManager` 类型存在，只持有 `fs`、`cmd`、`gh`、`svcMgr`
- [ ] `configManager` 类型存在，只持有 `fs`、`gh`、`ConfigValidator`
- [ ] 4 个构造函数存在（`NewServiceControl` 等）
- [ ] 原有的 `manager` 结构体和 `New()` 函数删除
- [ ] `go build ./...` 编译通过
- [ ] `go test ./...` 所有测试通过
- [ ] 无行为变更，纯重构

## Blocked by

- Issue 04（提取 Scheduler 模块）

## References

- 架构审查报告候选 4 — 拆分 manager god struct
