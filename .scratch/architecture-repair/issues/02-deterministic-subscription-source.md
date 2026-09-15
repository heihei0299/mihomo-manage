Status: resolved

## Parent

[architecture-repair](./01-architecture-repair.md)

## What to build

Make `subscription-source` deterministic for both remote and local subscription-data. When a user switches sources through the CLI or TUI, the selected source becomes authoritative, inactive source state is removed, and preview and subscription-update use only that source. Existing installations with exactly one legacy source migrate automatically; conflicting legacy sources require an explicit user choice instead of silent selection.

## Acceptance criteria

- [x] The user can select `remote` or `local` through the existing CLI and TUI configuration flows.
- [x] The selected source is persisted explicitly and is the only source consumed by preview and subscription-update.
- [x] Switching sources removes the inactive source state and cannot reuse a stale URL or subscription-data.
- [x] A legacy installation with exactly one source migrates without re-entering the source.
- [x] A legacy installation with both sources present reports a conflict and does not guess.
- [x] Missing or invalid source state produces an actionable result rather than silently selecting a source.
- [x] Tests cover source switching, unique migration, conflict migration, invalid state, preview, and subscription-update behavior.

## Implementation summary

Added explicit remote/local source state, fail-closed legacy migration, rollback on source-state write failure, stale-source protection, and a TUI editor entry point. Existing manager and TUI seams remain in use.

## Verification

- `go test ./internal/manager`
- `go test .`
- Scoped code review completed with no blocking or correctness findings after delta recheck.

## Blocked by

None - can start immediately
