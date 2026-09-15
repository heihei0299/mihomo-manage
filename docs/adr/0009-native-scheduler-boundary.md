# ADR-0009: Native scheduler package boundary

Status: Accepted

## Context

The manager package historically contained configuration, lifecycle, service,
and native scheduling implementation in one Go package. Logical ownership rules
reduced search cost, but systemd and launchd behavior remained reachable from
all manager files because package-private symbols are shared across the package.

The native scheduler had become a mature boundary:

- its public behavior was small and stable;
- systemd and launchd each had explicit state semantics;
- platform behavior had dedicated tests;
- the implementation did not need Config or Lifecycle internals.

The goal is to make this mature boundary compiler-visible without starting a
package-wide layered rewrite.

## Decision

Move native systemd and launchd platform mechanics to:

`internal/scheduler`

The scheduler package owns:

- systemd unit generation and state interpretation;
- launchd plist generation and runtime state interpretation;
- platform-specific Set, Stop, and Status behavior;
- the small filesystem and command-runner interfaces needed by those mechanics.

The dependency direction is strictly:

`internal/manager -> internal/scheduler`

`internal/scheduler` must not import `internal/manager`.

Manager retains:

- `ScheduleManager` orchestration;
- the mihomo-installed prerequisite;
- command-path selection;
- legacy schedule migration state;
- `LegacyScheduleError` and `UnsupportedPlatformError`;
- caller-facing scheduling semantics.

This keeps platform mechanics below manager while preserving business and
migration semantics at the orchestration boundary.

## Architecture enforcement

A lightweight standard-library architecture test protects:

- explicit ownership of manager production files;
- rejection of generic dumping-ground filenames;
- the one-way dependency from manager to scheduler.

The guard intentionally does not perform symbol-level dependency analysis.
Future domains should receive compiler-enforced boundaries only when their own
split triggers are met.

## Consequences

Positive:

- systemd and launchd internals can no longer be reached directly from unrelated
  manager code;
- scheduler tests and source retrieval become more localized;
- future scheduler changes have a smaller review surface;
- the first physical package extraction establishes a repeatable, minimal
  pattern for future mature domains.

Tradeoffs:

- scheduler defines small local side-effect interfaces that overlap with a
  subset of manager's interfaces;
- manager keeps an orchestration adapter around the extracted package;
- tests use package-local fakes rather than sharing manager fakes.

These are intentional costs to preserve one-way dependencies.

## Non-goals

This decision does not:

- extract Config;
- extract Lifecycle;
- extract Service;
- introduce dependency injection;
- introduce repository/usecase/controller layers;
- redesign the public schedule API;
- move caller-branchable schedule errors out of manager.

Further package extraction requires the split triggers documented in
`docs/maintainability.md`.
