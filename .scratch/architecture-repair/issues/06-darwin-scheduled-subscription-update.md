Status: blocked

## Parent

[architecture-repair](./01-architecture-repair.md)

## What to build

Provide Darwin parity for durable scheduled subscription-update through launchd. Reuse the ScheduleManager behavior contract established by the Linux slice while creating, inspecting, stopping, and removing a launchd scheduled task. The task must invoke the installed manager non-interactively, preserve the no-catch-up rule, and use the transactional subscription-update path.

## Acceptance criteria

- [x] The user can enable, inspect, and stop scheduled subscription-update through the existing CLI and TUI behavior.
- [x] Enabling a schedule creates or updates a launchd task with the configured interval.
- [x] The task invokes the installed manager with `subscription update --quiet` through an absolute path.
- [x] The schedule continues after the configuring CLI process exits.
- [x] Status distinguishes active, off, and unknown launchd state and does not turn query errors into off.
- [x] `subscription schedule --off` removes the launchd task and clears legacy schedule state.
- [x] Uninstalling the mihomo-instance removes the launchd scheduling artifacts.
- [x] Missed intervals are not replayed after host downtime.
- [x] Scheduled execution uses the transactional subscription-update path and reports failures through launchd logging.
- [ ] Unit tests cover the platform adapter and acceptance tests cover real launchd behavior. Unit tests pass; real launchd acceptance is unavailable on the Arch/systemd host.

## Implementation summary

Added the Darwin launchd system scheduler adapter using a system-level LaunchDaemon, bootstrap/bootout lifecycle, explicit system-domain status queries, stable manager path selection, interval persistence in the plist, and launchd stdout/stderr log paths.

## Verification

- `go test . ./internal/cli ./internal/manager` — 170 tests passed.
- Darwin adapter tests cover plist generation, bootstrap, status, query failure, and cleanup.
- Scoped code review completed after fixing launchd domain and error propagation.
- Real launchd acceptance is blocked because the available Arch container runs systemd, not launchd.

## Blocked by

- [02-deterministic-subscription-source.md](./02-deterministic-subscription-source.md)
- [03-transactional-subscription-update.md](./03-transactional-subscription-update.md)
- [05-linux-scheduled-subscription-update.md](./05-linux-scheduled-subscription-update.md)
