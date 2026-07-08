Status: ready-for-agent

## What to build

Wire the `ServiceControl` interface through CLI handler and TUI model, replacing direct `manager.Manager` calls for control operations.

### CLI handler changes

- `cli.Handler` gains a `control manager.ServiceControl` field
- `cli.New` constructor adds `ctrl manager.ServiceControl` parameter
- Handler methods `Status`, `Start`, `Stop`, `Restart`, `Reload`, `AutoStart` call `h.control` instead of `h.mgr`
- All other handler methods continue to use `h.mgr` (unchanged)

### TUI changes

- `model` gains a `control manager.ServiceControl` field
- `startTUI` adds `ctrl manager.ServiceControl` parameter
- TUI control commands (`actStart`, `actStop`, `actRestart`, `actReload`, `actAutostartOn`, `actAutostartOff`) call `m.control` instead of `m.mgr`

### `main.go` changes

- `cli.New` call passes `mgr` as first argument (implicit `ServiceControl` implementation)
- `startTUI` call passes `mgr` as first argument

## Acceptance criteria

- [ ] `cli.Handler` uses `h.control` for all 6 control methods
- [ ] TUI `model` uses `m.control` for all 6 control operations
- [ ] `mockControl` struct in `handler_test.go` covers the 6 methods (tests compile)
- [ ] `go build ./...` compiles
- [ ] `go test ./internal/cli/` passes
- [ ] CLI status, start, stop, restart, reload, autostart commands work identically

## Blocked by

- Issue 05 (role interfaces must exist)
