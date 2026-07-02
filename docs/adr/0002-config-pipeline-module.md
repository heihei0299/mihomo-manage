# Extract ConfigPipeline as a standalone deep module

## Context

The config-pipeline (CONTEXT.md §41) — the flow from subscription source through template rendering to final validated config — is currently scattered across 5 files (`config.go`, `system.go`, `schedule.go`, `lifecycle.go`, `manager.go`). Understanding the full flow requires cross-file navigation. Two seam leaks exist:

1. `config.go:136` calls `exec.Command()` directly for `mihomo -t` validation instead of going through `System.RunCommand()`
2. `config.go:148` calls `m.svcMgr.Reload()` directly, coupling the config pipeline to the service management layer

## Decision

Extract the config-pipeline into a standalone `ConfigPipeline` module with a small interface and a large implementation.

### Interface shape

```go
type ConfigPipeline interface {
    SetSubscriptionSource(ctx context.Context, source string) error
    SetRoutingRules(ctx context.Context, rules string) error
    Preview(ctx context.Context) (string, error)
    Apply(ctx context.Context) error
}
```

### Options for seam decoupling

- **Reload signal:** injected callback (`OnReload func(ctx) error`) rather than direct `ServiceManager` dependency. Chosen because the pipeline should not know about the service management layer.
- **Config validation:** separate `ConfigValidator` seam:

```go
type ConfigValidator interface {
    Validate(ctx context.Context, configPath string) error
}
```

Production implementation calls `mihomo -t`. Tests inject a no-op validator.

### Module boundary

- **In scope:** subscription fetch, template rendering, config backup, config validation, reload signaling.
- **Out of scope:** Install bootstrap (initial config/template writes remain in `lifecycle.go`), scheduled triggering (`schedule.go` calls `pipeline.Apply()` through the same `OnReload` callback).

## Consequences

- Config-pipeline logic becomes local to one module instead of 5 files
- Reload coupling removed from config logic
- `exec.Command` bypass fixed — validation goes through `ConfigValidator` seam and is testable
- Existing file path constants (`ConfigTemplatePath`, `binaryPath`, etc.) remain in `manager.go`
- Install bootstrap continues to write default config/template directly
