# 当前架构审查报告与执行计划

## 1. 审查信息

- **分支**：`main`
- **审查基线**：`63a81ea`
- **范围**：
  - `main.go`、`tui.go`
  - `internal/cli`
  - `internal/manager`
  - `internal/scheduler`
  - 架构 ADR、维护策略及相关测试
- **方式**：静态代码、调用关系和版本化文档审查
- **未执行**：编译、测试或行为修改

## 2. 总体结论

当前架构总体合理，不需要分层重写或全面拆包。

核心依赖关系为：

```text
main：装配、参数解析、进程生命周期
├── internal/cli.Handler
└── Bubble Tea TUI
       │
       ▼
ServiceControl / LifecycleManager / ConfigManager / ScheduleManager
       │
       ├── OSSystem：文件、命令、Release 外部适配器
       ├── OSServiceManager：systemd / launchd
       └── internal/scheduler：原生定时任务平台实现
```

已有架构的主要优点：

1. CLI 和 TUI 复用相同的业务角色接口。
2. Config、Lifecycle、Service、Schedule 已有明确所有权。
3. 配置应用与生命周期升级包含事务和回滚语义。
4. `internal/manager -> internal/scheduler` 的单向依赖由测试保护。
5. 外部依赖少，没有不必要的 DI、repository 或 usecase 层。
6. 架构文档与状态机测试较完整。

建议仅处理三个有明确收益的问题。

## 3. 审查发现

### R1：ConfigManager 构造函数存在隐藏 I/O 和不可见失败

**优先级：高**

#### 证据

`main.go:27-35` 在参数分发之前创建 `ConfigManager`：

```go
cfg := manager.NewConfigManager(...)
```

构造过程在 `internal/manager/config.go:83-105` 立即执行：

- 配置事务恢复；
- 旧配置文件迁移；
- 警告输出。

其中：

- `config.go:94` 直接写 `os.Stderr`；
- `config.go:101-104` 在构造时恢复和迁移；
- 恢复失败只通过 warning 输出，构造仍然成功。

#### 风险

1. `--help`、`--version` 等只读命令也可能访问或修改配置目录。
2. 初始化失败不能通过接口反馈给调用方。
3. manager 直接写终端，绕过 CLI writer、quiet 模式和 TUI 消息模型。
4. 构造函数同时承担装配和状态恢复，调用者难以判断副作用。

#### 建议

保持 `NewConfigManager` 为纯构造：

- 仅保存依赖；
- 不读写文件；
- 不获取锁；
- 不输出警告。

事务恢复和旧配置迁移放入私有准备流程，由真正依赖配置状态的公共操作调用；失败必须返回 `error`。

不要新增公开 `Initialize` 方法，避免给接口增加调用顺序约束。优先使用内部的 `prepare` / `prepareLocked` 流程。

#### 验收条件

- 调用 `NewConfigManager` 不产生文件、锁或输出操作。
- 未完成事务仍会在配置操作前恢复。
- 迁移失败作为错误返回，不再只打印警告。
- `internal/manager` 不直接写 `os.Stderr`。
- `--help` 和 `--version` 不接触 `/opt/mihomo*` 状态。

### R2：FileSystem 接口掩盖错误和破坏性语义

**优先级：高**

#### 证据

接口位于 `internal/manager/system.go:35-42`：

```go
FileExists(path string) bool
Remove(path string) error
```

实现中：

- `system.go:63` 的 `FileExists` 将所有 `os.Stat` 错误转换为 `false`；
- `system.go:76` 的 `Remove` 实际调用 `os.RemoveAll`。

调用位置覆盖 Lifecycle、Service、Config 和 Schedule。

#### 风险

1. 权限不足、路径损坏和真正不存在均被解释为“不存在”。
2. 调用方可能错误返回 `ErrMihomoNotInstalled`，掩盖真实系统错误。
3. `Remove` 的名称看不出它会递归删除目录。
4. 对 `/opt/mihomo` 等系统路径执行破坏性操作时，接口语义不够明确。

#### 建议

改为：

```go
FileExists(path string) (bool, error)
RemoveAll(path string) error
```

行为约束：

- 文件存在：`true, nil`；
- `fs.ErrNotExist`：`false, nil`；
- 权限、路径格式、I/O 等其他错误：`false, err`。

同时更新 `internal/scheduler` 的本地文件系统接口，以继续接受 `OSSystem` 适配器。

#### 验收条件

- 不存在与系统错误可以被调用方区分。
- Service、Lifecycle、Config、Schedule 不再吞掉文件状态错误。
- 所有递归删除调用点显式使用 `RemoveAll`。
- 不改变现有成功路径和删除范围。

### R3：关键依赖允许通过 nil 或可变参数静默禁用

**优先级：中**

#### 证据

Lifecycle 在 `internal/manager/lifecycle_impl.go:30` 使用可变参数接收 scheduler：

```go
func NewLifecycleManager(..., schedules ...ScheduleManager)
```

`Uninstall` 在 `lifecycle_impl.go:335` 仅当 scheduler 非空时清理任务。

Config 同样允许空依赖：

- `config.go:757`：validator 非空才校验；
- `config.go:781`：reload callback 非空才 reload。

#### 风险

生产装配当前正确，因此这不是已经发生的生产故障；但接口允许未来的错误装配正常编译并静默改变关键行为：

- 卸载可能遗留 systemd timer 或 launchd job；
- 配置可能未经校验直接提交；
- 配置提交后可能不 reload。

这些行为与 README 中的用户契约不一致。

#### 建议

1. `NewLifecycleManager` 使用普通必需参数：

   ```go
   NewLifecycleManager(fs, cmd, source, service, schedule)
   ```

2. 删除 scheduler 可变参数和 `nil` 分支。
3. validator、reload callback 和 `NewConfigValidator` 的 CommandRunner 设为必需依赖。
4. 测试需要跳过行为时，显式传入 no-op adapter，而不是 `nil`。
5. 依赖缺失按编程错误处理，与 `NewServiceControl` 当前策略保持一致。

#### 验收条件

- 正常构造的 LifecycleManager 一定拥有 ScheduleManager。
- 卸载一定先尝试移除定时任务。
- UpdateConfig 一定执行校验。
- 配置提交后一定尝试 reload。
- 测试中的可选行为通过明确的 no-op adapter 表达。

## 4. 暂不建议实施的改动

### 4.1 不全面拆分 `internal/manager`

虽然 `config.go`、`lifecycle_impl.go`、`servicemanager.go` 较大，但它们目前仍具备较好的接口深度和领域所有权，不应仅按行数拆包。

### 4.2 暂不提取 `internal/service`

`ServiceManager` 是下一个可能成熟的包边界，但服务文件生成仍被 Lifecycle 和 ServiceControl 使用：

- `lifecycle_impl.go:191,210,254`
- `service_impl.go:86`
- `servicemanager.go:18-90`

当前提取会迫使项目重新设计服务文件的所有权，收益尚不足。

仅在以下情况持续出现时再提取：

- 服务修改反复影响多个不相关的 manager 文件；
- 服务平台逻辑需要更多独立测试或适配器；
- Lifecycle 为访问服务内部 helper 而持续扩大共享区域。

### 4.3 不引入额外框架

暂不引入：

- DI 容器；
- repository/usecase/controller 层；
- 通用 command bus；
- Cobra 等 CLI 框架；
- 为 CLI/TUI 再增加一层统一 façade。

当前标准库参数解析和角色接口已经足够。

## 5. 执行计划

为减少重复修改，建议按以下顺序实施。

### 阶段 1：修正 FileSystem 语义

#### 修改范围

预计涉及：

- `internal/manager/system.go`
- `internal/manager/config.go`
- `internal/manager/lifecycle_impl.go`
- `internal/manager/service_impl.go`
- `internal/manager/native_scheduler.go`
- `internal/scheduler/platform.go`
- `internal/scheduler/linux.go`
- `internal/scheduler/darwin.go`
- 对应测试 fake

#### TDD 步骤

1. 添加失败测试：
   - 不存在路径返回 `false, nil`；
   - 非“不存在”错误能够传递；
   - Service/Lifecycle 不把文件系统错误转换成“未安装”。
2. 将 `FileExists` 改为 `(bool, error)`。
3. 更新调用方并增加操作上下文错误。
4. 将 `Remove` 机械重命名为 `RemoveAll`。
5. 更新 manager 和 scheduler 的测试 fake。

#### 局部验证

获得执行授权后运行：

```bash
go test ./internal/manager ./internal/scheduler
```

#### 完成标准

- 文件状态错误不再丢失；
- 所有递归删除在调用点可见；
- manager 与 scheduler 局部测试通过。

### 阶段 2：纯化 ConfigManager 构造过程

#### 修改范围

预计涉及：

- `internal/manager/config.go`
- `internal/manager/config_impl.go`
- 相关配置事务与迁移测试
- 必要时调整 `main.go` 的 warning 注入

#### TDD 步骤

1. 添加构造无副作用测试：
   - 不读取文件；
   - 不写文件；
   - 不 rename；
   - 不获取锁；
   - 不调用 warning sink。
2. 从 `newConfigPipeline` 移除恢复和迁移。
3. 添加私有准备流程，包括可锁定版本和已持锁版本。
4. 在依赖配置状态的入口执行准备流程。
5. 保留 UpdateConfig 当前事务恢复顺序。
6. 删除 manager 对 `os.Stderr` 的直接依赖。
7. 补充 `--help` / `--version` 无配置副作用测试。

#### 局部验证

获得执行授权后运行：

```bash
go test ./internal/manager -run 'Config|Transaction|Migration|Recovery'
go test . -run 'Help|Version'
```

#### 完成标准

- 构造函数完全无 I/O；
- 事务恢复和旧配置迁移行为不丢失；
- 初始化失败可观察；
- CLI/TUI 不再收到 manager 的旁路终端输出。

### 阶段 3：收紧关键依赖

#### 修改范围

预计涉及：

- `internal/manager/lifecycle_impl.go`
- `internal/manager/config_impl.go`
- `internal/manager/manager.go`
- `main.go`
- 相关测试构造调用

#### TDD 步骤

1. 增加卸载必须停止 schedule 的测试。
2. 将 Lifecycle scheduler 改为必需参数。
3. 删除 `m.schedule != nil` 分支。
4. 将 Config validator、reload callback 和 ConfigValidator runner 设为必需依赖。
5. 为测试添加最小 no-op adapter。
6. 更新所有构造调用；不添加 factory 或 DI 容器。

#### 局部验证

获得执行授权后运行：

```bash
go test ./internal/manager -run 'Lifecycle|Uninstall|Config|Validation'
go test ./internal/cli
go test .
```

#### 完成标准

- 错误装配不能静默关闭关键行为；
- 生产路径行为不变；
- 测试中的简化依赖均显式可见。

### 阶段 4：架构复核，不自动继续拆包

前三阶段完成后只做一次静态复核：

1. 检查 `internal/manager -> internal/scheduler` 仍为单向依赖。
2. 检查没有新增通用 helper 文件。
3. 检查 Config、Lifecycle、Service 所有权是否仍清晰。
4. 评估 Service 是否真正满足至少两个持续性拆包触发条件。

若没有新的跨领域耦合证据，阶段 4 的正确结果是停止，不拆包。

## 6. 建议提交拆分

建议保持三个独立提交，便于审查和回滚：

1. `refactor: make filesystem failure semantics explicit`
2. `refactor: remove config constructor side effects`
3. `refactor: require critical manager dependencies`

不新增依赖，不混入无关文档整理或格式修改。

## 7. 验证与授权说明

本报告阶段没有运行编译或测试。实施时建议先执行每阶段的局部测试；全量测试仅在全部修改完成且获得单独授权后执行。

## 8. 本次实施 compact review record

以下记录直接附在本执行计划中，不创建第二份平行报告；范围覆盖本次 Config、Lifecycle、Service 改动及其组合根 wiring。

### Config

- Owner: `internal/manager/config*.go`、`internal/manager/adopt.go`；状态/不变量所有权属于 `configPipeline`，包括事务恢复、旧配置迁移、校验、提交和 reload 结果。
- Production files inside owner: `internal/manager/config.go`、`internal/manager/config_impl.go`、`internal/manager/adopt.go`。
- Production files outside owner: `main.go` 仅负责组合依赖、warning sink 和 reload callback；`internal/manager/service_impl.go` 仅通过 `ConfigManager.ValidateConfig` 使用配置校验能力。
- Unrelated manager context required: no。
- Helper/type promoted to shared: no。
- New cross-domain dependency: no；使用既有 `ConfigValidator`、reload callback 和 `ConfigManager` role contract。
- Why this dependency is necessary: 配置提交必须校验并通知运行中的 mihomo，但 Config 不应直接依赖 Service 实现。
- Caller can use an existing role interface: yes；校验使用 `ConfigValidator`，reload 使用 `func(context.Context) error`，Service 使用 `ConfigManager`。
- Reverse dependency introduced or strengthened: no；依赖仍由组合根注入，Config 不反向导入 Service。

### Lifecycle

- Owner: `internal/manager/lifecycle*.go`；状态/不变量所有权属于 `lifecycleManager`，包括安装、卸载、升级、回滚以及失败后的服务恢复。
- Production files inside owner: `internal/manager/lifecycle_impl.go`。
- Production files outside owner: `main.go` 负责提供 schedule；`internal/manager/native_scheduler.go` 和 `internal/scheduler/*` 负责 schedule 的平台状态与实现。
- Unrelated manager context required: no。
- Helper/type promoted to shared: no。
- New cross-domain dependency: no；只是将已有 `ScheduleManager` role contract 从可选改为必需，并保持 `internal/manager -> internal/scheduler` 单向依赖。
- Why this dependency is necessary: 卸载必须先清理订阅定时任务，避免遗留 systemd timer 或 launchd job。
- Caller can use an existing role interface: yes；Lifecycle 只依赖 `ScheduleManager`，不访问 scheduler 平台内部。
- Reverse dependency introduced or strengthened: no；scheduler 不依赖 manager。

### Service

- Owner: `internal/manager/service*.go`、`internal/manager/servicemanager.go`；状态/不变量所有权属于 ServiceControl/ServiceManager，包括安装状态、运行状态、自启状态和服务控制操作。
- Production files inside owner: `internal/manager/service_impl.go`。
- Production files outside owner: `main.go` 仅组合 `ConfigManager.ValidateConfig`；`internal/manager/config*.go` 保有配置校验实现，不由 Service 访问其内部状态。
- Unrelated manager context required: no。
- Helper/type promoted to shared: no。
- New cross-domain dependency: no；继续使用既有 validation callback 和 `ServiceManager` role contract。
- Why this dependency is necessary: start/restart 必须在服务动作前验证生成配置，但 Service 不应拥有配置流水线。
- Caller can use an existing role interface: yes；通过 `func(context.Context) error` 注入校验能力。
- Reverse dependency introduced or strengthened: no；组合根持有并连接两侧，未形成 Config/Service 反向实现依赖。
