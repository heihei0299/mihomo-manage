# decouple-architecture — Work Plan (Draft)

## Metadata

- **slug**: decouple-architecture
- **intent**: clear
- **review_required**: true
- **status**: review_completed
- **plan_path**: .omo/plans/decouple-architecture.md
- **review_results**:
  - momus: APPROVED（minor notes: ActionDef and renderConfig two-hop — both fixed）
  - oracle: CHANGES_REQUESTED → 5 issues all fixed in plan
    - ✅ Fix 1: Phase 3 no longer deletes system.go early; deferred to Phase 4
    - ✅ Fix 2: renderConfig/hasTopLevelKeys no longer two-hop; stay in manager until Phase 6
    - ✅ Fix 3: schedule/ → schedmgr/ to avoid collision with scheduler/
    - ✅ Fix 4: F3 jq command corrected to check Imports (not ImportPath)
    - ✅ Fix 5: doc.go added for all new packages; go:embed/init() scanned in Phase 1; export surface noted
- **approach**: progressive package split — split `internal/manager/` into 4 focused packages (`domain/`, `config/`, `infra/`, `service/`, `schedule/`), centralize path constants, split OSSystem into independent types, maintain compile-at-every-step safety.

## Decisions (user-approved)

| # | Decision | Choice | Rationale |
|---|----------|--------|-----------|
| D1 | Package split depth | 渐进拆分 (4 packages) | 最小扰动，每步可编译 |
| D2 | Path constants | 集中到 `internal/config/` | 单一事实来源，避免散落 |
| D3 | DI injection style | 显式构造函数注入 | Go 惯用，类型安全 |
| D4 | Interface placement | 定义在 `internal/domain/` (consumer side) | 消费者只依赖接口，实现可替换 |
| D5 | Scope | 全量拆分，不移除功能 | Must-NOT-Have: 不改 CLI 行为、不改 TUI 行为、不修改业务逻辑 |

## Ledger

### Phase 1 — Ground truth

Explored every file in:
- `internal/manager/` (16 files) — all types, interfaces, implementations in one package
- `internal/cli/handler.go` — imports `manager` directly
- `internal/scheduler/scheduler.go` — well-decoupled, no changes needed
- `main.go` — does DI wiring + arg parsing + TUI launch
- `tui.go` — bubbletea model in `package main`

### Key coupling points identified

1. **Giant `manager` package**: 16 files, 1 package, mixes domain types + infrastructure + all implementations
2. **OSSystem god struct**: implements FileSystem + CommandRunner + GitHubReleases
3. **Hardcoded paths**: `/opt/mihomo/...` in 5+ files
4. **Cross-package concrete dep**: `manager/schedule_impl.go` imports `internal/scheduler`
5. **configValidator bypass**: uses raw `exec.Command` instead of injected `CommandRunner`
6. **cli depends on concrete package**: `cli/handler.go` imports `internal/manager`
7. **Package-level platform/env deps**: `serviceUnitPath()`, `releaseURL()` use `runtime.GOOS` and `os.Getenv()`

### Adopted defaults (not asked)

- Naming: `internal/infra/` (not `internal/infrastructure/`) for Go convention
- `ReleaseRepo` (not `GitHubReleases`) as interface name — more general
- All tests in `*_test.go` alongside implementation in new packages

## Approval gate

> **Status**: AWAITING APPROVAL
>
> **Brief**: This plan splits the monolithic `internal/manager/` package into 5 focused packages, centralizes path constants, and splits `OSSystem` into 3 independent types. 9 phased steps, each compilable. No behavior changes. No new features. All existing tests pass after each step.
>
> **Next action after approval**: Generate `.omo/plans/decouple-architecture.md` with full todo list

---

*Ready for your approval.*
