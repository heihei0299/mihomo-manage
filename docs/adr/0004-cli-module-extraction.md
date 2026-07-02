# Extract CLI handlers into a testable module

## Context

All 11 CLI handler functions live in package `main` (`main.go`). They write directly to `stdout`, call `os.Exit` on errors, and mix argument parsing with handler logic. Result: 429 lines of untestable code. No test file for package `main` exists.

## Decision

Extract a `cli` module (`internal/cli`) with exported `Handler` struct:

```go
package cli

type Handler struct {
    mgr    manager.Manager
    stdout io.Writer
    stderr io.Writer
}

func New(mgr manager.Manager, stdout, stderr io.Writer) *Handler
```

Each CLI command becomes a method on `Handler`, returning an exit code (`int`):

```
Status, Install, Uninstall, Start, Stop, Restart, Reload, Upgrade,
PreviewConfig, SetSubscription, UpdateConfig, Schedule, ScheduleStatus,
Versions, Logs
```

### Seam for testability

`Handler` accepts `io.Writer` for stdout and stderr, not `os.Stdout`/`os.Stderr` directly. Tests inject `bytes.Buffer` and assert on formatted output and exit codes.

### Boundary

- **In scope:** handler logic, output formatting, exit codes
- **Out of scope:** argument parsing (`main.go` maps parsed args to Handler method calls + `os.Exit`)

## Consequences

- All 11 handlers become testable via injected writers
- `main.go` shrinks to entry, wiring, argument parsing, and exit
- Existing quiet-mode (`quietPrintf`/`quietPrintln`) is replaced by the writer seam
- No change to `manager.Manager` interface
