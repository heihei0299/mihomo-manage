# Maintainability policy

This document defines the maintenance rules for the current architecture. The
goal is to reduce search cost, change blast radius, and state ambiguity without
adding architectural layers that the project does not need.

## Current package strategy

`internal/manager` remains one package for now. Logical boundaries are enforced
by file ownership and small interfaces before physical package splits are
considered.

### File ownership

| Area | Primary files | Owns |
| --- | --- | --- |
| Config | `config*.go`, `merge.go`, `adopt.go`, `lock.go` | subscription state, rendering, validation, transactional apply, adopt |
| Lifecycle | `lifecycle*.go` | install, upgrade, uninstall, rollback |
| Service | `service*.go`, `servicemanager.go` | service process control and platform service registration |
| Schedule | `native_scheduler.go`, `schedule_manager.go` | systemd/launchd scheduling and legacy schedule migration |
| OS seams | `system.go` | filesystem, command execution, release/network boundary implementations |
| Shared contract | `manager.go`, role interface files, `errors.go` | shared paths, public role contracts, caller-branchable errors |

Rules:

1. A helper belongs to the area whose invariant it protects. Do not place
   cross-domain helpers in a generic utility file.
2. Config internals must not call lifecycle internals; lifecycle may depend on
   role interfaces, not config implementation details.
3. Scheduler implementations own platform state interpretation. Callers must
   not infer systemd/launchd state from files.
4. Add interfaces only at replaceable side-effect boundaries or stable role
   boundaries. Internal pipeline stages remain ordinary functions.
5. Do not add repository/service/usecase/controller layers or a DI framework.

## State machines are executable contracts

State transitions that affect recovery or user-visible status must be documented
next to the code and covered by behavior tests.

Required matrices:

- Config apply: validation failure, pre-commit failure, pending reload, applied,
  and applied-with-cleanup-warning.
- launchd: runtime loaded/unloaded crossed with plist present/missing.
- Lifecycle: failures before deployment, after deployment, after registration,
  after replacement, and rollback failure propagation.

When a state is added, removed, or reinterpreted, update the code comment and
the corresponding tests in the same change.

## Errors

Only errors on which callers can reasonably branch should be sentinel or typed
errors. They live in `internal/manager/errors.go`.

Current branchable categories include:

- configuration update busy;
- subscription source not configured;
- mihomo not installed;
- adopt requiring confirmation;
- unsupported scheduler platform.

Operational errors remain wrapped with `fmt.Errorf("context: %w", err)`.
Do not create an error hierarchy for ordinary I/O or command failures.

## Boolean-return rule

A single boolean is acceptable when it has one obvious meaning such as
`active`. If a new API needs two or more independent booleans representing
runtime state, persistence state, health, or enablement, introduce a small
named result struct rather than positional booleans.

Existing APIs do not need churn solely to satisfy this rule.

## OSSystem growth limit

`OSSystem` currently implements three side-effect seams:

1. `FileSystem`
2. `CommandRunner`
3. `ReleaseSource`

Do not add a fourth responsibility to `OSSystem`. If process inspection,
archive handling, system information, or another independent capability is
needed, introduce a separate concrete implementation/seam rather than growing
`OSSystem`.

The current three implementations may remain on one concrete type while they
stay thin stdlib adapters.

## Tests

Prefer behavior matrices over implementation-detail assertions for recovery
logic. Important state combinations must be visible in test names or table
cases.

Do not run the full test suite after every small edit. During development, use
the narrowest relevant tests; run the full suite once at the final verification
gate when execution is authorized.

## Documentation hierarchy

- `README.md`: current user-visible behavior only. Do not hardcode the current
  release version.
- `docs/adr/`: durable architectural decisions and superseded history.
- `docs/repository-policy.md`: tracked repository assets and ignore policy.
- This file: current maintenance boundaries and split triggers.
- Review/remediation plans: execution records, not sources of truth for runtime
  behavior. Once completed, they should not be extended with new product
  semantics.

If runtime behavior conflicts with an old plan, update the current source of
truth instead of treating the plan as authoritative.

## Package split triggers

Do not split packages based on line count alone. Consider a physical split only
when at least two of these signals persist across multiple changes:

1. Three or more consecutive changes in one area repeatedly touch four or more
   unrelated manager files.
2. Code search for one area routinely returns substantial unrelated config,
   lifecycle, service, or scheduler implementation.
3. An area has a stable public contract and tests that rarely need package-wide
   internals.
4. Reviewers repeatedly need unrelated manager context to validate a localized
   change.
5. Circular ownership pressure appears: helpers are moved to shared files only
   so two areas can reach them.

When a split is justified, extract one mature area at a time. Prefer `config`
or `schedule` first if their independence is demonstrated. Keep orchestration
small; do not perform a package-wide layered rewrite.

## AI-assisted retrieval

For a localized task, inspect the owning area first and expand only when an
explicit dependency requires it. Do not preload the entire `internal/manager`
package for a config-only, scheduler-only, or lifecycle-only change.

This rule is intended to improve both human navigation and AI context/cache
efficiency.
