Status: resolved

## Parent

[architecture-repair](./01-architecture-repair.md)

## What to build

Make scheduled subscription-update durable on Linux through systemd while preserving one-shot manager execution. The CLI and TUI must configure, inspect, and stop the schedule; the native task must invoke the installed manager through an absolute path in quiet mode; systemd must be the source of active state and interval; and disabling or uninstalling the mihomo-instance must remove the task. Legacy schedule state is reported for explicit migration and never activates a task during a read operation.

## Acceptance criteria

- [x] The user can enable the fixed TUI presets and the CLI interval through the existing ScheduleManager behavior.
- [x] Enabling a schedule creates or updates a systemd timer and its associated execution task.
- [x] The execution task invokes the installed manager with `subscription update --quiet` through an absolute path.
- [x] The schedule continues after the configuring CLI process exits.
- [x] Status distinguishes active, off, and unknown systemd state and does not turn query errors into off.
- [x] `subscription schedule --off` removes the systemd task and clears legacy schedule state.
- [x] Uninstalling the mihomo-instance removes the systemd scheduling artifacts.
- [x] Missed intervals are not replayed after host downtime.
- [x] Legacy `schedule.txt` is reported without creating a task as a side effect of status or another read operation.
- [x] Scheduled execution uses the transactional subscription-update path and reports failures through systemd logging.
- [x] Unit tests cover the platform adapter and acceptance tests cover real systemd behavior.

## Implementation summary

Replaced the production in-process scheduler with Linux systemd service/timer integration, stable installed-manager command path selection, native active-state/interval querying, legacy schedule reporting, CLI/TUI fixed presets, and uninstall cleanup. The old ticker scheduler was removed.

## Verification

- `go test . ./internal/cli ./internal/manager` — 167 tests passed.
- Native scheduler adapter tests cover unit generation, status, stop cleanup, query failure, and legacy state.
- In the Arch Incus container, `TestAcceptanceSchedule` passed, including enabled timer, absolute `ExecStart`, `--off` cleanup, and status behavior.
- In the Arch Incus container, `TestAcceptanceUninstall` passed, including native schedule removal.
- Scoped code review completed with no remaining blocking or correctness findings.
- The full legacy acceptance suite still contains unrelated upstream URL/version assumptions; those failures are not part of this issue's scheduler path.

## Blocked by

- [02-deterministic-subscription-source.md](./02-deterministic-subscription-source.md)
- [03-transactional-subscription-update.md](./03-transactional-subscription-update.md)
