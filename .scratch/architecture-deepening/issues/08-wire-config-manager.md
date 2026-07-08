Status: ready-for-agent

## What to build

Wire the `ConfigManager` interface through CLI handler and TUI model, replacing `manager.Manager` calls for subscription and config operations.

### CLI handler changes

- `cli.Handler` gains a `config manager.ConfigManager` field
- `cli.New` constructor adds `config manager.ConfigManager` parameter
- Handler methods `SetSubscription`, `UpdateConfig`, `PreviewConfig` call `h.config` instead of `h.mgr`
- All other handler methods continue to use `h.mgr`

### TUI changes

- `model` gains a `config manager.ConfigManager` field
- `startTUI` adds `config manager.ConfigManager` parameter
- TUI config operations (`fetchConfigPreview`, `execActionCmd` → `m.mgr.PreviewConfig`) use `m.config`

### `main.go` changes

- `cli.New` call passes `mgr` as additional argument
- `startTUI` call passes `mgr` as additional argument

## Acceptance criteria

- [ ] `cli.Handler` uses `h.config` for SetSubscription, UpdateConfig, PreviewConfig
- [ ] TUI uses `m.config` for config preview
- [ ] `mockConfig` struct in `handler_test.go` covers the 4 methods
- [ ] `go build ./...` compiles
- [ ] `go test ./internal/cli/` passes
- [ ] CLI subscription set/update and config preview commands work identically

## Blocked by

- Issue 05 (role interfaces must exist)
