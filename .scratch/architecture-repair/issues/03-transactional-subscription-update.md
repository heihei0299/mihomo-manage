Status: resolved

## Parent

[architecture-repair](./01-architecture-repair.md)

## What to build

Make manual and scheduled `subscription-update` safe to run through the same end-to-end configuration path. Serialize concurrent updates across processes, stage and validate generated configuration before committing it atomically, preserve a recoverable backup, and expose the latest configuration application result through CLI and TUI status. A reload failure must retain the validated generated configuration while returning a failed result. External commands and downloads in this path must honor cancellation and propagate critical errors.

## Acceptance criteria

- [x] Concurrent manual and scheduled updates cannot enter the configuration commit path at the same time.
- [x] A contending update waits only for a bounded period and then reports a clear busy result.
- [x] Generated configuration is written to a staged location, validated there, and atomically committed only after validation succeeds.
- [x] The previous generated configuration is backed up before a successful commit.
- [x] Validation failure leaves the previously committed configuration in place and records `validation-failed`.
- [x] Reload failure retains the new generated configuration, returns failure, and records `pending-reload`.
- [x] A fully successful update records `applied`.
- [x] The application result includes timestamp, attempted configuration hash, and short error summary, and corrupted state is reported as unknown.
- [x] CLI and TUI expose the latest application result without confusing it with `instance-state`.
- [x] Cancellation reaches downloads, validation, and service commands; critical write, validation, replacement, and reload errors are not swallowed.
- [x] Tests cover locking, staging, validation, backup, atomic replacement, reload failure, state transitions, corrupted state, cancellation, and CLI/TUI output.

## Implementation summary

Added bounded OS file locking, staged config validation with atomic replacement and backups, persisted JSON application status, CLI/TUI status display, context-aware command/service seams, cancellation-safe lifecycle rollback, and critical error propagation.

## Verification

- `go test . ./internal/cli ./internal/manager` — 161 tests passed.
- Scoped code review completed with no remaining blocking or correctness findings after delta rechecks.
- Global `git diff --check` still reports an unrelated pre-existing blank line at EOF in `.gitignore`; scoped changed files are clean.

## Blocked by

- [02-deterministic-subscription-source.md](./02-deterministic-subscription-source.md)
