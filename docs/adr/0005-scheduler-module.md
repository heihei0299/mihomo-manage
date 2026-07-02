# Extract Scheduler as a self-contained module

## Context

The subscription update scheduler (`schedule.go`) manages a goroutine with `time.Ticker` that periodically calls `UpdateConfig`. Its lifecycle state (`mu`, `ticker`, `stopCh`) lives as fields on the `manager` struct. The interface (3 methods) is shallow — implementation is also 3 methods. The goroutine silently swallows errors from `UpdateConfig`:

```go
case <-ticker.C:
    m.UpdateConfig(context.Background())  // error discarded
```

## Decision

Extract a `Scheduler` module that encapsulates goroutine lifecycle, persistent state, and error handling:

```go
type Scheduler interface {
    Start(ctx context.Context, interval time.Duration, task func(context.Context)) error
    Stop(ctx context.Context) error
    Status(ctx context.Context) (interval time.Duration, active bool, err error)
}
```

### Ownership

- **Persistence:** `Scheduler` internally manages the schedule file (read interval, write interval, write `"off"`). Callers don't touch the file.
- **File access:** `Scheduler` receives a `FileSystem` seam for its persistence.
- **Error handling:** task execution errors are reported via a callback (for logging), but the scheduler continues running. Errors are not silently discarded.

### Rationale

- Goroutine lifecycle fields move out of `manager` struct into the scheduler module
- Callers pass a callback (`func(ctx)`) — the scheduler has no knowledge of the manager or config pipeline
- Error swallowing (`schedule.go:38`) is fixed

## Consequences

- `manager` struct loses `mu`, `ticker`, `stopCh` fields
- `manager.SetSchedule` / `StopSchedule` / `ScheduleStatus` delegate to the scheduler module
- Existing `schedule.go` file is replaced by the new module
- Schedule persistence behavior is unchanged (same file format, same path)
