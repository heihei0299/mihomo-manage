# ADR-0011: Linux and Windows platform support

Status: Accepted

## Decision

Support Linux and Windows, with amd64 and arm64 releases for each. Remove the
macOS service and scheduler implementations and its release targets. Other
operating systems return an explicit unsupported error before runtime setup.

Linux retains systemd service management, systemd timers, POSIX permissions,
`flock`, and journalctl logs.

Windows uses the Service Control Manager through `golang.org/x/sys/windows`.
SCM runs the installed manager with `service-run`; this host starts the core,
writes its stdout and stderr to the managed log, handles stop/shutdown, and
reports unexpected child exits as service failures. A Windows Job Object kills
the core when its host disappears. SCM restarts failed hosts after five seconds.
Reload stops and starts the service because Windows does not provide SIGHUP.

Windows Task Scheduler runs subscription updates as SYSTEM. Registered task
definitions determine runtime status; staged XML does not imply registration.
Stopping a schedule stops running task instances and deletes the registered task.
Intervals range from one hour to 31 days and use whole seconds.

Windows stores the core and config under `%ProgramData%\mihomo`, state and logs
under `%ProgramData%\mihomo-manager`, and the stable lock outside both uninstall
roots under `%ProgramData%\mihomo-manager-locks`. LockFileEx serializes writers.
Private storage uses protected ACLs granting access to its owner, Administrators,
and SYSTEM. Windows releases are ZIP archives; local ZIP, gzip, and raw binary
installation use the same staging and lifecycle rollback paths.

Windows mutations and the TUI require an Administrator terminal. The default
editor is Notepad; command parsing preserves quoted executable paths. Logs read
the core log file and support tail and follow.

## Validation and ownership

CI runs Go tests and vet on Linux and Windows. Release builds cover all four
platform/architecture combinations. Native Windows tests exercise private ACLs,
file locking, and hosted process stop/failure behavior without registering real
system services. Scheduler tests cover registered, missing, disabled, stale XML,
query failure, and deletion failure states using the existing command seam.

Owner: Service, Lifecycle, Scheduler, shared paths/locking, OS seams, CLI bootstrap.
Production files outside these owners: none; frontend wiring calls their contracts.
Unrelated manager context required: no.
Helper promoted to shared: platform paths and file lock primitives only.
New cross-domain dependency: Lifecycle can check a production service registration
target before deploying, to avoid deleting an unrelated SCM service on rollback.
State owner: Service owns SCM registration and child process state; Scheduler owns
registered task state; Lifecycle owns install/upgrade rollback.
Existing role contract: ServiceManager retains normal service control; the optional
registration-target check belongs to the production service boundary.
Reverse dependency introduced: no; Scheduler continues to have no Manager imports.
