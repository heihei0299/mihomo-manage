# Extract ConfigPipeline as a standalone deep module

## Status

accepted; current override-file semantics are defined by ADR-0007. The original `ConfigPipeline` interface was superseded by M3; `ConfigManager` is now the caller-facing contract.

## Context

The config-pipeline (CONTEXT.md §41) — the flow from subscription source through override merging to final validated config — was extracted from the manager orchestration. The module now owns preview, staging, validation, atomic commit, reload signaling, and apply-status recording behind focused seams.

## Decision

Extract the config-pipeline into a standalone deep module with a large implementation. It remains inside `internal/manager` as `configPipeline`; callers use `ConfigManager`, and no separate `ConfigPipeline` interface is maintained.

### Historical interface shape (superseded by M3)

```go
type ConfigPipeline interface {
    SetSubscriptionSource(ctx context.Context, source string) error
    Preview(ctx context.Context) (string, error)
    Apply(ctx context.Context) error
    LastConfigApply(ctx context.Context) (ConfigApplyStatus, error)
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

- **In scope:** subscription fetch, override merging, config backup, config validation, reload signaling, and apply-status recording.
- **Out of scope:** Install bootstrap (initial config/override writes remain in `lifecycle.go`), scheduled triggering (the schedule manager invokes the caller-facing config update operation through the same `OnReload` callback).

## Consequences

- Config-pipeline logic becomes local to one module instead of 5 files
- Reload coupling removed from config logic
- `exec.Command` bypass fixed — validation goes through `ConfigValidator` seam and is testable
- Existing file path constants (`OverrideFilePath`, `binaryPath`, etc.) remain in `manager.go`
- Install bootstrap continues to write the default config/override files directly
