Status: completed

# mihomo-manager v1.0 代码稳定化

## Parent

`.scratch/mihomo-manager/PRD.md`

## Problem Statement

当前代码库已实现全部功能，但多轮代码审查揭示了若干结构性气味：重复的 OS 切换 switch、重复的下载-解压流程、职责混杂的大文件、新增功能需修改多处。在 v1.0 发布前，需要降低这些结构的维护成本——不影响功能行为，只改代码结构。

## Solution

对代码库进行 5 项结构性改进，全部是纯重构（行为不变，测试不改绿——除了测试 seam 本身的调整）。

## User Stories

1. As a developer, I want servicemanager.go 的 7 个重复 `switch s.goos()` 合并为一次分派，so that 新增操作系统支持时只需改一处
2. As a developer, I want Install 和 Upgrade 中的下载-解压-清理重复逻辑提取为共享方法，so that 下载逻辑的变更不会遗漏一处
3. As a developer, I want manager.go 中的生命周期操作、配置管理、调度、版本解析拆分为独立文件，so that 一个职责的变更不涉及无关代码
4. As a developer, I want 新增一个控制操作（如 pause）时只需改 action 定义 + 实现 2 处而非 4 处，so that 减少遗漏
5. As a developer, I want `RenderConfig` 取消导出（`renderConfig`），so that 调用者不产生该函数是公共 API 的错觉
6. As a developer, I want `simpleOp` 和 `assertError` 更名为表意名称，so that 代码即文档

## Implementation Decisions

### 1. OS 切换策略模式

将 `servicemanager.go` 中 7 个方法的 `switch s.goos()` 替换为策略接口：

```go
type osStrategy interface {
    isActive(name string) (bool, error)
    enable(name string) error
    disable(name string) error
    start(name string) error
    stop(name string) error
    restart(name string) error
    reload(name string) error
}
```

两个实现：`linuxStrategy`（systemctl）和 `darwinStrategy`（launchctl）。`OSServiceManager` 在构造时根据 `goos()` 选策略。新增 OS 只需加一个新策略实现。

### 2. 下载-解压提取

当前 `Install` 和 `Upgrade` 各自实现了一段几乎相同的下载 `.gz` → 解压 → 清理临时文件的逻辑。提取为 manager 的内部方法 `downloadAndDecompress(ctx, url, dest)`。

### 3. 按职责拆分 manager.go

当前 `manager.go` ~700 行，混杂：

- 生命周期（Install/Uninstall/Upgrade）→ `lifecycle.go`
- 配置管理（PreviewConfig/UpdateConfig/SetSubscriptionSource）→ `config.go`
- 定时调度（SetSchedule/StopSchedule）→ `schedule.go`
- 实例控制（Start/Stop/Restart/Reload）→ `control.go`
- 类型定义 + 版本解析 → 留在 `manager.go`

所有方法仍是 `manager` struct 的接收者，接口不变——纯文件拆分。

### 4. 动作注册集中化

当前 TUI 中新增一个操作需改：`action` 常量定义、`execActionCmd` 的 switch、`isActionAllowed` 的 switch、`View` 的 switch。改为动作注册表：

```go
type actionDef struct {
    key      string
    label    string
    enabled  func(*Status) bool
    execute  func(Manager) error
}
```

新增操作只需在注册表中添加一项。

## Testing Decisions

- **同一 seam**：测试继续通过 Manager 接口 + mockSystem，行为不变则测试不改
- **servicemanager 策略测试**：新策略实现复用现存的 `commandRecorder` 测试模式，每个 OSStrategy 独立测试
- **无行为变更**：重构期间测试全绿，确认行为保持

## Out of Scope

- 新增功能（不在已有 PRD 中的用户故事）
- 重写 TUI 的标签页布局
- 新增 CLI 命令

## Further Notes

- 本 issue 不引入新功能，只重构
- 每个子项可独立 PR
- 建议顺序：1（OS 策略）→ 2（下载提取）→ 4（动作注册）→ 3（文件拆分）→ 5/6（命名清理），因为 3 依赖其他几项重构完后再拆文件
