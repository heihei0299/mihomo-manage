# Maintainability policy

This document defines the maintenance rules for the current architecture. The
goal is to reduce search cost, change blast radius, and state ambiguity without
adding architectural layers that the project does not need.

## Current package strategy

`internal/manager` remains the orchestration package. Logical domains stay
inside manager until a physical split has demonstrated value. Mature platform
implementations may be extracted one domain at a time; the native scheduler is
the first compile-time boundary.

### File ownership

| Area | Primary files | Owns |
| --- | --- | --- |
| Config | `config*.go`, `merge.go`, `adopt.go`, `lock.go` | subscription state, rendering, validation, transactional apply, adopt |
| Lifecycle | `lifecycle*.go` | install, upgrade, uninstall, rollback |
| Service | `service*.go`, `servicemanager.go` | service process control and platform service registration |
| Schedule orchestration | `native_scheduler.go`, `schedule_manager.go` | installed prerequisite, legacy schedule migration, platform selection, caller-facing errors |
| Native scheduler | `internal/scheduler/*` | systemd/launchd generation, native runtime state interpretation, platform scheduling |
| OS seams | `system.go` | filesystem, command execution, release/network boundary implementations |
| Shared contract | `manager.go`, role interface files, `errors.go` | shared domain contracts, public role contracts, caller-branchable errors |
| Shared paths | `paths.go` | genuinely cross-domain filesystem paths and shared filesystem permissions |
| Bootstrap defaults | `defaults.go` | generated install/bootstrap override and config content |

Rules:

1. A helper belongs to the area whose invariant it protects. Do not place
   cross-domain helpers in a generic utility file.
2. Config internals must not call lifecycle internals; lifecycle may depend on
   role interfaces, not config implementation details.
3. Native scheduler implementations own systemd/launchd state interpretation.
   Callers must not infer platform state from persisted files.
4. Dependency direction is one-way: `internal/manager` may depend on
   `internal/scheduler`; `internal/scheduler` must not import
   `internal/manager`.
5. Add interfaces only at replaceable side-effect boundaries or stable role
   boundaries. Internal pipeline stages remain ordinary functions.
6. Do not add repository/service/usecase/controller layers or a DI framework.

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
- mihomo not installed / already running / not running;
- adopt requiring confirmation;
- legacy scheduler configuration;
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

## Architecture guards

`internal/manager/architecture_test.go` protects the lowest-cost structural
rules that are not naturally enforced by the current package layout.

It must remain small and standard-library-only. Its responsibilities are:

- every manager production file has an explicit domain owner;
- generic dumping-ground names such as `utils.go`, `common.go`, or
  `constants.go` are rejected;
- `internal/scheduler` cannot import `internal/manager`.

Do not grow this into a symbol-level dependency analyzer. When a logical
boundary needs compiler-enforced symbol isolation, extract that mature domain
into its own package instead.

## Tests

Prefer behavior matrices over implementation-detail assertions for recovery
logic. Important state combinations must be visible in test names or table
cases.

Shared fakes should expose only behavior reused by most tests. Keep one-off
blocking or failure behavior in a specialized fake beside the test group that
needs it; do not grow a general-purpose God Fake.

Do not run the full test suite after every small edit. During development, use
the narrowest relevant tests; run the full suite once at the final verification
gate when execution is authorized.

## Change review evidence

For each meaningful Config, Lifecycle, or Service change, use the following
compact review record to capture its change blast radius. For each new
cross-domain dependency, the record must also answer the four dependency
questions below before merging:

- A same-domain helper stays in its owning area.
- A cross-domain call uses an existing stable role contract when one already
  represents the required capability.
- Do not introduce an interface solely to hide an otherwise unnecessary
  dependency.
- Do not move a helper to a shared file merely so two areas can reach it.

Before adding a symbol to `manager.go`, a role interface file, `errors.go`,
`paths.go`, or `defaults.go`, verify that:

- it is genuinely needed by more than one ownership area or is a caller-facing
  contract;
- its semantics are not owned by one specific domain;
- moving it to shared does not bypass an existing ownership boundary;
- it does not add mutable cross-domain state.

```text
Owner:
Production files inside owner:
Production files outside owner:
Unrelated manager context required: yes/no
Helper/type promoted to shared: yes/no
New cross-domain dependency: yes/no
State or invariant owner:
Why this dependency is necessary:
Caller can use an existing role interface: yes/no; if no, why:
Reverse dependency introduced or strengthened: yes/no
```

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

When a split is justified, extract one mature area at a time. The native
scheduler extraction is the reference pattern: keep orchestration and
caller-facing semantics in manager, move mature platform mechanics behind a
small one-way boundary, and avoid a package-wide layered rewrite.

Do not extract Config, Lifecycle, or Service merely for symmetry.

## AI-assisted retrieval

For a localized task, inspect the owning area first and expand only when an
explicit dependency requires it. Do not preload the entire `internal/manager`
package for a config-only, scheduler-only, or lifecycle-only change.

Preferred first-read boundaries:

- **Config:** `config*.go`, `merge.go`, `adopt.go`, `lock.go`, and the matching `config_*_test.go` files. Expand to service, lifecycle, or scheduler only for an explicit call dependency.
- **Schedule orchestration:** `native_scheduler.go`, `schedule_manager.go`, and manager schedule tests.
- **Native scheduler:** `internal/scheduler/*` only. Expand to manager only when reviewing the package boundary or constructor wiring.
- **Lifecycle:** `lifecycle*.go`, service role contracts, and the matching lifecycle tests. Expand only when the lifecycle path calls another area.

This rule is intended to improve both human navigation and AI context/cache
efficiency.
