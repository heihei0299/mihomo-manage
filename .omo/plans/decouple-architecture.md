# decouple-architecture - Work Plan

## TL;DR (For humans)

**What you'll get**: 从当前的巨型 `internal/manager` 包（16 个文件混在一起）拆分为 6 个高内聚低耦合的包（`domain/`, `config/`, `infra/`, `service/`, `schedmgr/`），`OSSystem` 拆为 3 个独立类型，硬编码路径集中管理，CLI 仅依赖接口。

**Why this approach**: 渐进式拆分确保每步都可编译运行，不改任何行为逻辑。采用 Go 惯用的构造函数注入和消费者侧接口定义。

**What it will NOT do**: 不改变 CLI/TUI 行为，不修改业务逻辑，不加新功能，不修改 `scheduler/`。

**Effort**: 9 个阶段，每阶段约 5-15 个文件修改。预计执行时间 1-2 次 worker session。

**Risk**: 低——每步编译验证，测试全部通过后推进下一步。

## Scope

**In scope**:
- 拆分 `internal/manager/` → `domain/`, `config/`, `infra/`, `service/`, `schedmgr/`
- `OSSystem` → `OSFileSystem` + `OSCommandRunner` + `GitHubRepo`
- 路径常量集中到 `internal/config/paths.go`
- 接口定义集中到 `internal/domain/interfaces.go`
- 更新所有 import 路径
- 更新测试文件

**Out of scope**:
- 不改变 CLI/TUI 行为
- 不修改业务逻辑
- 不修改 `scheduler/`（底层 ticker）
- 不加新功能/命令
- 不改 `go.mod` 依赖

## Verification strategy

每阶段末尾运行：
```bash
go build ./...
go vet ./...
go test ./internal/... -count=1
```
阶段 9 额外运行：
```bash
go test ./... -count=1 -tags=acceptance  # if acceptance tests exist
```

## Execution strategy

9 个阶段，每个阶段创建新文件 + 修改旧文件。每阶段结束时项目可编译，测试通过。
阶段 1-3 创建基础包（零依赖包优先），阶段 4-6 迁移实现（`service/`, `schedmgr/`, config pipeline），阶段 7-8 更新消费者，阶段 9 清理。

## Todos

### Wave 1 — 创建基础零依赖包

- [x] 1. 创建 `internal/config/` 包 — 集中路径常量

    **References**:
    - `internal/manager/manager.go` lines 148-166: 所有 `const` 路径定义
    - `internal/manager/lifecycle.go` lines 9-14: `serviceUnitPath()` 中的路径
    - `internal/manager/lifecycle.go` line 71: `defaultReleaseTemplate`
    - `internal/manager/go.mod`: module path `github.com/anomalyco/mihomo-manager`

    **Actions**:
    1. 创建 `internal/config/paths.go`：
       - package `config`
       - 从 `manager/manager.go` 移动所有路径常量：`binaryPath`, `configDir`, `ConfigTemplatePath`, `configYAML`, `defaultServiceUnitPath`, `ServiceName`, `stateDir`, `subscriptionDataFile`, `subscriptionURLFile`, `RoutingRulesPath`, `scheduleFile`, `filePermUserRW`, `filePermUserRWX`
       - 从 `manager/lifecycle.go` 移动 `defaultReleaseTemplate`
       - **导出所有常量（首字母大写）** — 这是刻意为之：允许其他包通过 `config.BinaryPath` 访问而不是用字面量。虽然扩大了导出面，但这是集中管理的目的所在。
    2. 为每个新包创建 `doc.go`（或包级别注释）：
       - `internal/config/doc.go`: `// Package config provides centralized path constants and the config pipeline for mihomo.`
       - `internal/domain/doc.go`: `// Package domain defines core types and interfaces for mihomo-manager.`
       - `internal/infra/doc.go`: `// Package infra provides filesystem, command, and release infrastructure.`
       - `internal/service/doc.go`: `// Package service implements service control and lifecycle management.`
       - `internal/schedmgr/doc.go`: `// Package schedmgr manages periodic subscription updates.`
    3. 创建 `internal/config/defaults.go`：
       - 从 `manager/manager.go` 移动 `defaultTemplate` 和 `defaultConfig` 变量
    3. 更新 `internal/manager/` 中所有文件以 import `config` 包并使用 `config.BinaryPath` 等
       - 受影响文件：`manager.go`, `lifecycle.go`, `lifecycle_impl.go`, `service_impl.go`, `servicemanager.go`, `config.go`, `config_impl.go`, `schedule_impl.go`

    **Acceptance**: `go build ./...` 通过；所有路径引用使用 `config.XXX` 而非字面量

    **QA**:
    - Happy: `go build ./... && go vet ./...`
    - Failure: 故意写错常量名 → 编译失败验证未使用的旧常量已被移除
    - 额外确认：grep 扫描 `internal/manager/` 中无残留 `//go:embed` 指令（已验证：当前项目无 go:embed，仅在 Phase 1 确认一次即可）。同时确认 `internal/manager/system.go` 中的 `init()` 是唯一一个 `init()`，将在 Phase 3 迁移。

    **Commit**: `refactor: extract internal/config/ package with centralized path constants`

- [x] 2. 创建 `internal/domain/` 包 — 纯类型 + 所有接口

    **References**:
    - `internal/manager/manager.go`: `InstanceState`, `InstallationPhase`, `ProgressEvent`, `ProgressCallback`, `VersionInfo`, `Status`
    - `internal/manager/service_control.go`: `ServiceControl` interface
    - `internal/manager/lifecycle_manager.go`: `LifecycleManager` interface
    - `internal/manager/config_manager.go`: `ConfigManager` interface
    - `internal/manager/schedule_manager.go`: `ScheduleManager` interface
    - `internal/manager/config.go`: `ConfigValidator`, `ConfigPipeline` interfaces
    - `internal/manager/servicemanager.go`: `ServiceManager` interface

    **Actions**:
    1. 创建 `internal/domain/types.go`：
       - `InstanceState` + `String()`
       - `InstallationPhase` + `String()`
       - `ProgressEvent`
       - `ProgressCallback`
       - `VersionInfo`
       - `Status`
    2. 创建 `internal/domain/interfaces.go`：
       - `ServiceControl`
       - `LifecycleManager`
       - `ConfigManager`
       - `ScheduleManager`
       - `ConfigValidator`
       - `ConfigPipeline`
       - `ServiceManager`
       - （注意：`FileSystem`, `CommandRunner`, `ReleaseRepo` 在 `infra/` 中定义，不在这里）
     3. 创建 `internal/domain/util.go`：
        - 从 `manager/manager.go` 移动 `looksLikeURL()`, `looksLikeVersion()`, `parseVersion()`, `timestamp()`
        - ⚠️ **不要移动 `renderConfig()` 和 `hasTopLevelKeys()`** — 它们与 config 实现耦合，留在 `manager/config.go`，Phase 6 直接迁移到 `internal/config/`
    4. 更新 `internal/manager/` 中所有文件以 import `domain/` 并使用 `domain.Status` 等
       - 受影响文件：所有 manager 文件使用这些类型的地方
    5. 更新 `internal/cli/handler.go`：import `domain/` 替代 `manager/` 中的类型引用
    6. 更新 `tui.go` 和 `main.go`：import `domain/`

    **Acceptance**: `go build ./...` 通过；所有类型和接口引用来自 `domain.XXX`

    **QA**:
    - Happy: `go build ./... && go vet ./...`
    - Failure: 尝试在 `domain/` 中引用 `internal/manager` → 编译失败（确保无循环依赖）

    **Commit**: `refactor: extract internal/domain/ package with types and interfaces`

- [x] 3. 创建 `internal/infra/` 包 — 基础设施接口 + OS 实现

    **References**:
    - `internal/manager/system.go`: `FileSystem`, `CommandRunner`, `GitHubReleases` interfaces + `OSSystem` struct

    **Actions**:
    1. 创建 `internal/infra/filesystem.go`：
       - `FileSystem` interface（从 system.go 移动，不改方法签名）
       - `OSFileSystem` struct（OSSystem 中与文件系统相关的方法）
       - `FileExists`, `ReadFile`, `WriteFile`, `Remove`, `Rename`, `MkdirAll`, `Chmod`
    2. 创建 `internal/infra/command.go`：
       - `CommandRunner` interface
       - `OSCommandRunner` struct
       - `RunCommand`, `RunCommandIgnoreExit`
    3. 创建 `internal/infra/release.go`：
       - `ReleaseRepo` interface（重命名 `GitHubReleases`，更通用）
       - `GitHubRepo` struct（原先 OSSystem 中的 Download, ListVersions, LatestVersion）
     4. 创建 `internal/infra/client.go`：
        - 移动 `init()` 中的 `downloadClient` 设置
        - 只初始化一次（`sync.Once`）
     5. 更新 `main.go` 中的构造函数（但暂不删除 `manager/system.go`）：
        - `main.go`: `infra.OSFileSystem{}` / `infra.OSCommandRunner{}` / `infra.GitHubRepo{}`
        - 注意：`main.go` 改为使用 `infra.*` 类型，但 `internal/manager/` 内部仍使用旧的 `OSSystem`
     6. ⚠️ **不删除 `internal/manager/system.go`** — 暂保留它因为 `manager/servicemanager.go` 仍引用 `OSSystem`。在 Phase 4 迁移完 `servicemanager.go` 后删除。

    **Acceptance**: `go build ./...` 通过；不再有 `OSSystem` 类型，三个独立类型各司其职

    **QA**:
    - Happy: `go build ./... && go test ./internal/infra/... -count=1`
    - Failure: 移除 `OSSystem` 后编译检查残留引用

    **Commit**: `refactor: extract internal/infra/ with FileSystem, CommandRunner, ReleaseRepo`

### Wave 2 — 迁移实现到新包

- [x] 4. 创建 `internal/service/` 包 — 服务管理 + 生命周期
    （同时：在 Phase 4 结束时删除 `internal/manager/system.go`，因为 `servicemanager.go` 已迁出，`manager/` 不再引用 `OSSystem`）

    **References**:
    - `internal/manager/servicemanager.go`: `osStrategy`, `linuxSystemctl`, `darwinLaunchctl`, `OSServiceManager`
    - `internal/manager/service_impl.go`: `serviceController`
    - `internal/manager/lifecycle_impl.go`: `lifecycleManager`
    - `internal/manager/lifecycle.go`: `serviceUnitPath()`, `serviceUnitContent()`, `releaseURL()`

    **Actions**:
    1. 创建 `internal/service/servicemanager.go`：
       - `ServiceManager` 实现：`OSServiceManager` + `osStrategy` + `linuxSystemctl` + `darwinLaunchctl`
       - 接口 `ServiceManager` 已在 `domain/` 中，`service/` 实现它
    2. 创建 `internal/service/controller.go`：
       - `serviceController` 实现 `domain.ServiceControl`
    3. 创建 `internal/service/lifecycle.go`：
       - `lifecycleManager` 实现 `domain.LifecycleManager`
    4. 创建 `internal/service/serviceunit.go`：
       - `serviceUnitPath()`, `serviceUnitContent()`, `releaseURL()` 移到这里
       - 使用 `runtime.GOOS` 包装成可测试函数签名
    5. 更新 `main.go` 中的构造函数：
       - 从 `manager.NewServiceControl(...)` → `service.NewController(...)`
       - 从 `manager.NewLifecycleManager(...)` → `service.NewLifecycle(...)`
       - 从 `manager.NewOSServiceManager(...)` → `service.NewOSServiceManager(...)`
    6. 移动测试文件：`lifecycle_test.go`, `servicemanager_test.go` → `internal/service/`，更新 import
    7. **删除 `internal/manager/system.go`** — `servicemanager.go` 已迁出，`manager/` 不再引用 `OSSystem`。如果删除时报编译错误，说明还有残留引用，需先修复。

    **Acceptance**: `go build ./...` 通过；`manager` 包不再包含服务管理代码

    **QA**:
    - Happy: `go test ./internal/service/... -count=1`
    - Failure: 验证 `manager` 中已无 `osStrategy`、`serviceController` 等类型

    **Commit**: `refactor: extract internal/service/ package for service management and lifecycle`

- [x] 5. 创建 `internal/schedmgr/` 包 — 定时调度

    **说明**: 包名 `schedmgr`（schedule manager 缩写）避免与已有的底层 ticker `internal/scheduler/` 混淆。Go 惯用简短无歧义包名。

    **References**:
    - `internal/manager/schedule_impl.go`: `scheduleManager`
    - `internal/manager/schedule_manager.go`: `ScheduleManager` interface（已移到 domain/）

    **Actions**:
    1. 创建 `internal/schedmgr/manager.go`：
       - package `schedmgr`
       - `scheduleManager` 实现 `domain.ScheduleManager`
       - import `internal/scheduler`（具体依赖，同前）
    2. 更新 `main.go`：`manager.NewScheduleManager(...)` → `schedmgr.NewManager(...)`
    3. 删除 `internal/manager/schedule_impl.go` 和 `internal/manager/schedule_manager.go`

    **Acceptance**: `go build ./...` 通过；`manager` 包不再包含调度代码

    **QA**:
    - Happy: `go test ./internal/schedmgr/... -count=1`
    - Failure: 验证 `internal/schedmgr/` 不依赖 `internal/manager/`

    **Commit**: `refactor: extract internal/schedmgr/ package for schedule manager`

- [x] 6. 移动 ConfigPipeline 到 `internal/config/`

    **References**:
    - `internal/manager/config.go`: `configPipeline`, `configValidator`, `ConfigPipelineOptions`, `renderConfig`, `hasTopLevelKeys`
    - `internal/manager/config_impl.go`: `configManager` 实现 `domain.ConfigManager`
    - `internal/manager/manager.go` 中 `NewConfigValidator()`

    **Actions**:
    1. 在 `internal/config/pipeline.go` 中创建：
       - `configPipeline` → `pipeline`（实现 `domain.ConfigPipeline`）
       - `configValidator` → `validator`（实现 `domain.ConfigValidator`）
       - `ConfigPipelineOptions`
       - `renderConfig()`, `hasTopLevelKeys()`（从 domain 移到这里，因为它们与 config 实现耦合）
    2. 在 `internal/config/manager.go` 中创建：
       - `manager` struct 实现 `domain.ConfigManager`
    3. 更新 `main.go` 中的构造函数：
       - `manager.NewConfigManager(...)` → `config.NewManager(...)`
       - `manager.NewConfigValidator()` → `config.NewValidator()`
    4. 删除 `internal/manager/config.go`, `config_impl.go`, `config_manager.go`

    **Acceptance**: `go build ./...` 通过；`manager` 包不再包含配置管道代码

    **QA**:
    - Happy: `go test ./internal/config/... -count=1`
    - Failure: 验证 `internal/config/` 不依赖 `internal/manager/`

    **Commit**: `refactor: move config pipeline to internal/config/ package`

### Wave 3 — 更新消费者 + 清理

- [x] 7. 更新 `internal/cli/handler.go` — 仅依赖 domain 接口

    **References**:
    - `internal/cli/handler.go` 当前 imports `internal/manager`
    - 实际使用的类型：`manager.ServiceControl`, `manager.LifecycleManager`, `manager.ConfigManager`, `manager.ScheduleManager`, `manager.ProgressEvent`, `manager.ProgressCallback`, `manager.InstallationPhase`
    - 这些全部已在 `internal/domain/` 中

    **Actions**:
    1. 将 `import "github.com/anomalyco/mihomo-manager/internal/manager"` 替换为 `import ".../internal/domain"`
    2. 将 `manager.ServiceControl` → `domain.ServiceControl` 等全部替换
    3. `Handler` struct 字段类型同样更新
    4. `New()` 函数参数类型更新

    **Acceptance**: `go build ./...` 通过；`cli` 不再 import `manager`

    **QA**:
    - Happy: `go test ./internal/cli/... -count=1`
    - Failure: grep 验证 `internal/cli/` 中无 `internal/manager` 引用

    **Commit**: `refactor: update cli to depend only on domain interfaces`

- [x] 8. 更新 `main.go` 和 `tui.go` — 新包 wiring

    **References**:
    - `main.go` 当前进行所有 DI wiring，import `internal/manager` 和 `internal/cli`
    - `tui.go` 在 `package main` 中直接使用 `manager.*` 类型

    **Actions**:
    1. 更新 `main.go`：
        - 替换所有 import：
          - `manager` → `domain`, `config`, `infra`, `service`, `schedmgr`
       - 更新 `main()` 中的构造函数调用：
         ```go
         // 旧
         oss := &manager.OSSystem{}
         svcMgr := manager.NewOSServiceManager(oss, oss)
         ctrl := manager.NewServiceControl(oss, oss, svcMgr)
         lifecycle := manager.NewLifecycleManager(oss, oss, svcMgr)
         cfg := manager.NewConfigManager(oss, oss, manager.NewConfigValidator(), ...)
         sched := manager.NewScheduleManager(oss, ...)
         
         // 新
         fs := &infra.OSFileSystem{}
         cmd := &infra.OSCommandRunner{}
         release := &infra.GitHubRepo{}
         svcMgr := service.NewOSServiceManager(cmd, fs)
         ctrl := service.NewController(fs, cmd, svcMgr)
         lifecycle := service.NewLifecycle(fs, cmd, release, svcMgr)
         cfg := config.NewManager(fs, release, config.NewValidator(), ...)
          sched := schedmgr.NewManager(fs, ...)
         ```
       - `cli.New(ctrl, lifecycle, cfg, sched, ...)` 不变（参数类型是接口，自动适配）
    2. 更新 `tui.go`：
       - 替换所有 `manager.*` 类型引用为 `domain.*`
       - 注意 `tui.go` 中使用 `manager.ActionDef`, `manager.InstallationPhase`, `manager.VersionInfo` 等
    3. TUI 的 `startTUI()` 函数签名更新

    **Acceptance**: `go build ./...` 通过；全部编译，`main` 包不再 import `manager`

    **QA**:
    - Happy: `go build ./... && go vet ./...`
    - Failure: grep 验证无残留 `internal/manager` import

    **Commit**: `refactor: update main.go and tui.go wiring for new packages`

- [x] 9. 清理 — 删除 `internal/manager/` 包 + 更新测试

    **References**:
    - `internal/manager/` 目录（应仅剩空包或完全可删除）
    - `internal/manager/manager_test.go`, `internal/manager/manager.go`（可能残留共享工具函数）

    **Actions**:
    1. 检查 `internal/manager/` 中是否还有未被移动的文件：
       - `manager_test.go` — 如果测试了 `looksLikeURL` 等，移到 `internal/domain/util_test.go`
       - 确保所有 `go:embed`、`init()`、包级变量已迁移
    2. 删除 `internal/manager/` 目录
    3. 创建/更新测试文件：
       - 确保 `internal/domain/types_test.go` 测试类型 String() 方法
       - 确保 `internal/infra/*_test.go` 测试各基础设施组件
    4. 全局 grep 确认无 `import ".../internal/manager"` 残留
    5. 运行完整测试套件

    **Acceptance**: `internal/manager/` 目录不存在；`go test ./... -count=1` 全部通过

    **QA**:
    - Happy: `go build ./... && go test ./... -count=1`
    - Failure: grep `internal/manager` 返回 0 匹配

    **Commit**: `refactor: remove internal/manager/ package`

## Final verification wave

- [x] F1. 完整编译 + Vet 检查
    - `go build ./...` 无错误
    - `go vet ./...` 无警告
- [x] F2. 全量测试通过
    - `go test ./... -count=1` 全部通过（含 acceptance 测试）
- [x] F3. 包依赖图验证
    - **(a)** 确认无人导入 `internal/manager`：
      ```bash
      go list -json ./... | jq -r 'select(.Imports[] | contains("internal/manager")) | .ImportPath'
      ```
      该命令应输出空（无任何包引用了已删除的 `manager` 包）。
    - **(b)** 验证每个新包的 import 图符合预期：
      - `domain/` 不导入任何 `internal/` 包
      - `infra/` 不导入任何 `internal/` 包
      - `config/` 可导入 `domain/`, `infra/`
      - `service/` 可导入 `domain/`, `config/`, `infra/`
      - `schedmgr/` 可导入 `domain/`, `infra/`, `scheduler/`
      - `cli/` 仅导入 `domain/`
- [x] F4. 行为不变性检查
    - 运行 `go run . version` 确认版本输出正常
    - 运行 `go run . --help` 确认帮助输出与之前一致

## Commit strategy

每个阶段一个独立 commit，前缀均为 `refactor:`，每个 commit 后项目可编译且测试通过。

建议 worker 使用 `--make-pr` 按阶段依次应用提交，PR 标题："refactor: decouple monolitic manager package into focused packages"

## Success criteria

1. `internal/manager/` 包完全删除
2. 所有 `go build`、`go vet`、`go test` 通过
3. CLI 所有命令输出与重构前一致
4. TUI 界面与交互行为与重构前一致
5. 无新依赖加入 `go.mod`
6. 包间依赖图无循环依赖
