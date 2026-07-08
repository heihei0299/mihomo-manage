Status: ready-for-agent

## What to build

Remove the now-unused `Manager` interface from `internal/manager/manager.go`. Clean up `main.go` and `tui.go` wiring.

### Manager package

- Delete the `Manager interface` block (18 methods)
- `New()` returns `*manager` instead of `Manager`

### `main.go`

- Remove `mgr` variable usage; each caller gets its own interface variable implicitly via the `cli.New` and `startTUI` constructor arguments

### `tui.go`

- `model` struct: remove `mgr manager.Manager` field (no longer needed — control, lifecycle, and config fields already exist)
- `startTUI` signature: already takes 3 role interfaces — confirmed no unused parameters
- Remove any remaining `m.mgr` references in model methods (should be zero if Slices 1-3 were complete)
- TUI testability is now enabled: each role interface can be mocked independently (3 small mocks vs 1 large mock of 18 methods)

### `internal/cli/handler.go`

No change needed — `Handler.mgr` was already removed in Issue 09.

## Acceptance criteria

- [ ] `Manager interface` deleted from `manager.go`
- [ ] `New()` returns `*manager`
- [ ] No compilation errors in any package
- [ ] `go test ./...` passes
- [ ] `main.go` no longer imports or references `manager.Manager` type
- [ ] Binary builds and runs correctly

## Blocked by

- Issue 06 (ServiceControl)
- Issue 07 (LifecycleManager)
- Issue 08 (ConfigManager)
- Issue 09 (ScheduleManager)
