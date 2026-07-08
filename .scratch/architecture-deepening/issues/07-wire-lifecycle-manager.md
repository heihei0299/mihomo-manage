Status: ready-for-agent

## What to build

Wire the `LifecycleManager` interface through CLI handler and TUI model, replacing `manager.Manager` calls for lifecycle operations.

### CLI handler changes

- `cli.Handler` gains a `lifecycle manager.LifecycleManager` field
- `cli.New` constructor adds `lifecycle manager.LifecycleManager` parameter
- Handler methods `Install`, `Uninstall`, `Upgrade`, `Versions` call `h.lifecycle` instead of `h.mgr`
- All other handler methods continue to use `h.mgr`

### TUI changes

- `model` gains a `lifecycle manager.LifecycleManager` field
- `startTUI` adds `lifecycle manager.LifecycleManager` parameter
- TUI lifecycle commands (`actInstall`, `actUninstall`, `actUpgrade`) call `m.lifecycle` instead of `m.mgr`
- `fetchVersionsCmd` also uses `m.lifecycle`

### `main.go` changes

- `cli.New` call passes `mgr` as additional argument
- `startTUI` call passes `mgr` as additional argument

## Acceptance criteria

- [ ] `cli.Handler` uses `h.lifecycle` for all 4 lifecycle methods
- [ ] TUI `model` uses `m.lifecycle` for install/uninstall/upgrade/versions
- [ ] `mockLifecycle` struct in `handler_test.go` covers the 4 methods
- [ ] `go build ./...` compiles
- [ ] `go test ./internal/cli/` passes
- [ ] CLI install, uninstall, upgrade, versions commands work identically

## Blocked by

- Issue 05 (role interfaces must exist)
