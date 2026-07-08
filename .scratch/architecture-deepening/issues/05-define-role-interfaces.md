Status: ready-for-agent

## What to build

Define four role interfaces in `internal/manager/` that decompose the current `Manager` interface into narrower seams. No behavioral changes — this is a pure additive prefactor.

Four new files:

**`internal/manager/service_control.go`**

```go
type ServiceControl interface {
    Status(ctx context.Context) (*Status, error)
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Restart(ctx context.Context) error
    Reload(ctx context.Context) error
    SetAutoStart(ctx context.Context, enabled bool) error
}
```

**`internal/manager/lifecycle_manager.go`**

```go
type LifecycleManager interface {
    Install(ctx context.Context, version string, autoStart bool, onProgress ProgressCallback) error
    Uninstall(ctx context.Context, keepBackup bool, onProgress ProgressCallback) error
    Upgrade(ctx context.Context, version string, onProgress ProgressCallback) error
    ListVersions(ctx context.Context) ([]VersionInfo, error)
}
```

**`internal/manager/config_manager.go`**

```go
type ConfigManager interface {
    SetSubscriptionSource(ctx context.Context, source string) error
    SetRoutingRules(ctx context.Context, rules string) error
    PreviewConfig(ctx context.Context) (string, error)
    UpdateConfig(ctx context.Context) error
}
```

**`internal/manager/schedule_manager.go`**

```go
type ScheduleManager interface {
    SetSchedule(ctx context.Context, interval time.Duration) error
    StopSchedule(ctx context.Context) error
    ScheduleStatus(ctx context.Context) (time.Duration, bool, error)
}
```

The existing `Manager` interface and all callers remain untouched. The `*manager` struct already implements all four interfaces (since it implements `Manager` which is a superset).

## Acceptance criteria

- [ ] Four new interface files added under `internal/manager/`
- [ ] `go build ./...` compiles without errors
- [ ] `go test ./...` passes (no behavioral change)
- [ ] Existing code references only `manager.Manager` — new interfaces not yet wired

## Blocked by

None — can start immediately.
