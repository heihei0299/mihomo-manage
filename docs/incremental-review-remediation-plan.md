# Incremental Review Remediation Plan

Status: completed

This document is a historical execution record. Current maintenance rules are defined in `docs/maintainability.md`; do not use this plan as the source of truth for current architecture or runtime behavior.

## Scope

This document captures the follow-up remediation plan from the incremental review after:

- `b995669` — `fix(schedule): make native scheduler operations idempotent`
- `70e748c` — `fix(config): distinguish apply and validation failures`
- `236df3f` — `refactor(config): split apply pipeline stages`
- `309d089` — `chore(repo): rename release source and clarify tracking policy`

The remaining work is intentionally limited to three issues. Do not expand this pass into a broad architecture refactor.

---

## Ticket 1 — Decouple launchd job state from plist state

### Goal

Do not infer whether a launchd job is loaded from whether the plist file exists.

These are independent states:

```text
plist exists  != job loaded
plist missing != job unloaded
```

The scheduler must handle all four combinations correctly.

### Required changes

Add an internal launchd state query, for example:

```go
func (s *darwinPlatformScheduler) isLoaded(ctx context.Context) (bool, error)
```

Use:

```text
launchctl print system/mihomo-manager-subscription-update
```

Expected semantics:

```text
print succeeds                      -> loaded = true
explicit not-found / not-loaded     -> loaded = false
any other launchctl error           -> return error
```

Do not use `FileExists(launchdSchedulePlist)` to decide whether to call `bootout`.

### Set flow

Target flow:

```text
validate interval
    -> query launchd job state
    -> if loaded: bootout
    -> write plist
    -> bootstrap
```

This must work even when the plist is missing but the job is still loaded.

### Stop flow

Target flow:

```text
query launchd job state
    -> if loaded: bootout
    -> remove plist if present
    -> success
```

Required behavior matrix:

```text
job unloaded + plist missing  -> success
job unloaded + plist exists   -> remove stale plist
job loaded   + plist missing  -> bootout, then success
job loaded   + plist exists   -> bootout, then remove plist
```

Keep the current `bootout()` tolerance for explicit "not loaded" errors as race protection. For example, a job can disappear between `isLoaded()` and `bootout()`.

Do not swallow unrelated `launchctl` failures.

### Tests

Add or update tests for:

```text
Set:
[x] plist missing + job loaded
[x] plist missing + job unloaded
[x] plist exists + job loaded
[x] plist exists + job unloaded

Stop:
[x] plist missing + job loaded
[x] plist missing + job unloaded
[x] plist exists + job loaded
[x] plist exists + job unloaded

Errors:
[x] launchctl print real failure is propagated
[x] bootout real failure is propagated
[x] explicit not-loaded race remains idempotent
```

The existing `TestDarwinPlatformSchedulerStopWithoutPlist` must no longer assert that a missing plist means launchctl is never queried. The scheduler must query actual job state first.

### Acceptance criteria

- filesystem state and launchd runtime state are treated independently;
- `Set()` and `Stop()` are idempotent for all four state combinations;
- stale plist files can be repaired;
- orphan loaded jobs can be stopped;
- real launchctl errors are still surfaced.

### Suggested commit

```text
fix(schedule): decouple launchd job and plist state
```

---

## Ticket 2 — Preserve applied state when only post-commit cleanup fails

### Goal

Distinguish:

```text
configuration was not applied
```

from:

```text
configuration was applied successfully, but cleanup failed afterward
```

A post-commit cleanup failure must not be reported as `apply-failed` after the new config has already been committed and reloaded successfully.

### State model

Treat the apply flow as three phases:

```text
A. pre-commit
B. commit + activation
C. post-commit cleanup
```

#### A. Pre-commit failures

Examples:

```text
source resolution
download
preview generation
staging
backup
rename preparation
```

State:

```text
apply-failed
```

Validator-specific failures remain:

```text
validation-failed
```

#### B. Commit / activation

```text
rename staged config -> config.yaml
reload mihomo
```

State mapping:

```text
rename failure                    -> apply-failed
rename success + reload failure   -> pending-reload
rename success + reload success   -> applied
```

#### C. Post-commit cleanup

Examples:

```text
remove staging directory
remove temporary artifacts after successful commit
```

If commit and reload have already succeeded, cleanup failure must not change the state to `apply-failed`.

### Recommended minimal behavior

Do not add another public state in this pass.

Record:

```text
State: applied
ErrorSummary: cleanup error
```

For example:

```go
ConfigApplyStatus{
    State:        ConfigApplied,
    ErrorSummary: "cleanup staged config: ...",
}
```

Keep the current external error return behavior for now:

```text
return cleanup error
stored state = applied
```

This gives callers a warning/error signal while still accurately reporting the machine state.

Do not change `pending-reload` semantics.

### Tests

Add or update tests for:

```text
[x] rename failure -> apply-failed
[x] validation failure -> validation-failed
[x] reload failure -> pending-reload
[x] successful commit + reload + cleanup failure -> applied
[x] cleanup warning is preserved in ErrorSummary
[x] cleanup failure still follows the existing API error-return contract
[x] clean success -> applied with empty ErrorSummary
```

Update the current staging cleanup test so it expects:

```text
ConfigApplied
```

rather than:

```text
ConfigApplyFailed
```

### Acceptance criteria

Final state semantics:

```text
validation failure                 -> validation-failed
pre-commit non-validation failure -> apply-failed
commit success + reload failure   -> pending-reload
commit + reload success           -> applied
commit + reload success
  + post-commit cleanup failure   -> applied + ErrorSummary
```

### Suggested commit

```text
fix(config): preserve applied state on cleanup failure
```

---

## Ticket 3 — Restore required repository infrastructure

### Goal

Make the actual repository contents match `docs/repository-policy.md`.

The ignore policy is now corrected, but files deleted by `37bc3fc` were not automatically restored.

Do not restore every deleted development artifact. Restore only repository infrastructure that is still part of the intended project.

### Candidate assets to inspect

At minimum inspect the versions from before `37bc3fc` for:

```text
README.md
LICENSE
.github/workflows/release.yml
docs/acceptance-tests.md
docs/adr/*
```

### Category A — Restore by default

Unless intentionally retired, restore:

```text
README.md
LICENSE
.github/workflows/release.yml
```

These are core project assets:

```text
README              -> project entry point
LICENSE             -> distribution/legal metadata
release workflow    -> release automation
```

### Category B — Review and restore if still valid

Inspect:

```text
docs/acceptance-tests.md
docs/adr/*
```

If an ADR describes a decision that has since changed, prefer restoring it and marking it superseded rather than silently losing the architectural history.

If documentation is materially outdated, update it as part of the restore instead of copying stale content unchanged.

### Category C — Do not automatically restore

Do not restore these solely because they were previously tracked:

```text
.agents/
.pi/
.mcp.json
local workspace journals
local skill metadata
other AI-development-only artifacts
```

Only restore them if the current project workflow explicitly depends on them.

### Validation before restoring

For `README.md`, verify at least:

```text
CLI commands still exist
paths are still correct
scheduler behavior is current
config override behavior is current
version references are not stale
```

For `.github/workflows/release.yml`, verify at least:

```text
Go version
build matrix
artifact naming
release paths
tag trigger
Linux/Darwin targets
```

### Repository policy hardening

Update `docs/repository-policy.md` to list required tracked assets explicitly, for example:

```md
## Required repository assets

- README.md
- LICENSE
- .github/workflows/release.yml
- docs/
```

Do not add a repository-wide catch-all ignore rule again.

### Acceptance criteria

The current repository should contain, at minimum:

```text
README.md
LICENSE
.github/
```

and:

- the release workflow is compatible with the current project;
- the README is aligned with current CLI/runtime behavior;
- relevant ADRs and acceptance documentation are restored after review;
- `.gitignore` remains explicit and narrow;
- unrelated AI/local workspace files are not restored automatically.

### Completion

- [x] Required README, LICENSE, `.github/`, acceptance documentation, and ADRs are restored.
- [x] README, workflow, and restored documentation are aligned with current CLI, scheduler, config, and license behavior.
- [x] Local agent and workspace artifacts remain outside the restoration.

### Suggested commit

```text
chore(repo): restore required repository infrastructure
```

If README synchronization is substantial, split it:

```text
chore(repo): restore repository infrastructure
docs: align README with current behavior
```

---

## Execution order

Follow this order:

```text
1. launchd state model
2. config applied/cleanup semantics
3. repository infrastructure restore
```

The first two are runtime correctness issues.
The third is repository maintainability and completeness.

Keep them in separate commits.

---

## Final review gate

After all three tickets are complete, perform one incremental review from the current reviewed baseline to the new HEAD.

Verify:

```text
[x] launchd covers all four plist/job state combinations
[x] FileExists is no longer used as the loaded-state authority
[x] post-commit cleanup failure no longer produces apply-failed
[x] pending-reload semantics remain unchanged
[x] README / LICENSE / release workflow are actually restored
[x] no unrelated package split / DI framework / broad architecture rewrite was introduced
```

## Execution record

- Ticket 1: completed with `fix(schedule): decouple launchd job and plist state`.
- Ticket 2: completed with `fix(config): preserve applied state on cleanup failure`.
- Ticket 3: completed with `chore(repo): restore required repository infrastructure`.
- Verification: `go test ./internal/manager` (160 tests), `go test .` (13 tests), asset/policy checks, and final Standards/Spec review passed.
- No force push, package split, dependency-injection framework, or local AI artifacts were introduced.

Once these checks pass, this maintainability remediation round can be considered complete.
