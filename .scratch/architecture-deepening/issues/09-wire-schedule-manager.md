Status: ready-for-agent

## What to build

Wire the `ScheduleManager` interface through CLI handler, replacing the last `h.mgr` usage. After this slice, `cli.Handler.mgr` becomes unused and is removed.

### CLI handler changes

- `cli.Handler` gains a `schedule manager.ScheduleManager` field
- `cli.New` constructor adds `schedule manager.ScheduleManager` parameter
- Handler methods `SetSchedule`, `StopSchedule`, `ScheduleStatus` call `h.schedule` instead of `h.mgr`
- All remaining handler methods now use the role-specific fields
- Delete the `h.mgr` field from `Handler` struct — no callers remain

### Tests

- Add `mockSchedule` struct covering the 3 schedule methods
- All existing CLI handler tests continue to pass with the new constructor signature (5 arguments: control, lifecycle, config, schedule, stdout, stderr)

### TUI

No change — TUI does not use schedule operations.

### `main.go`

- `cli.New` call passes `mgr` as fourth (schedule) argument

## Acceptance criteria

- [ ] `cli.Handler.mgr` field deleted (no compile errors)
- [ ] `cli.Handler` uses `h.schedule` for all 3 schedule methods
- [ ] `mockSchedule` struct in `handler_test.go` covers the 3 methods
- [ ] `go build ./...` compiles
- [ ] `go test ./internal/cli/` passes
- [ ] CLI schedule status/set/stop commands work identically

## Blocked by

- Issue 05 (role interfaces must exist)
