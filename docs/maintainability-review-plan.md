# Maintainability Review and Execution Plan

## Scope

This document summarizes the maintainability-focused review of the recent changes in `heihei0299/mihomo-manage`, especially the commits from 2026-09-15 to 2026-09-16.

The goal is not to perform a large architecture rewrite. The recommended direction is a small, low-risk maintainability pass that improves correctness, diagnosis, readability, and long-term change cost.

## Review Recommendations

### 1. Fix launchd idempotency first

`darwinPlatformScheduler.Set()` and `Stop()` should not fail merely because a plist exists while the launchd job is not currently loaded.

Target behavior:

```text
plist exists + job loaded      -> unload/update normally
plist exists + job not loaded  -> continue update/remove
plist missing + job not loaded -> idempotent success
```

Do not ignore all `launchctl bootout` failures. Only treat the explicit "not loaded / not found" state as idempotent success. Real launchctl failures must still be returned.

### 2. Correct ConfigApply state semantics

The current apply path can classify non-validation failures as `validation-failed`.

Recommended minimal state model:

```go
const (
    ConfigApplied          = "applied"
    ConfigValidationFailed = "validation-failed"
    ConfigApplyFailed      = "apply-failed"
    ConfigPendingReload    = "pending-reload"
)
```

Expected mapping:

```text
source/download/write/stage/backup/rename failure -> apply-failed
validator failure                                 -> validation-failed
rename success + reload failure                   -> pending-reload
reload success                                    -> applied
```

This improves production diagnosis and avoids misleading state.

### 3. Reduce the cognitive complexity of configPipeline.Apply()

Do not introduce more public interfaces or change the external API.

Split only meaningful private stages, for example:

```go
refreshSubscription()
buildPreview()
stageAndValidate()
commitConfig()
reloadAndRecord()
```

The purpose is to make `Apply()` read like orchestration rather than implementation detail.

Preserve the existing transactional ordering:

```text
lock
 -> refresh
 -> preview
 -> stage
 -> validate
 -> backup
 -> atomic rename
 -> reload
 -> status record
 -> unlock
```

Take care not to break temporary file cleanup, staging cleanup, backup timing, rename atomicity, status persistence, or defer ordering.

### 4. Revisit the repository cleanup strategy

The current repository-wide ignore strategy:

```gitignore
*
!*/
!*.go
!go.mod
!go.sum
!.gitignore
```

is risky from a maintenance perspective because repository infrastructure can disappear silently.

Prefer ignoring generated artifacts explicitly, for example:

```gitignore
/bin/
/dist/
/build/
*.tmp
*.log
coverage.out
```

At minimum, decide explicitly whether the repository should preserve:

```text
README*
LICENSE*
.github/
docs/
*.md
```

If the repository is intentionally source-only, document that policy clearly.

### 5. Rename GitHubReleases to ReleaseSource

The abstraction is no longer GitHub-specific because custom mirrors are supported.

Prefer:

```go
type ReleaseSource interface {
    Download(...)
    ExpectedChecksum(...)
    ListVersions(...)
    LatestVersion(...)
}
```

This is a small, low-risk naming improvement that makes the architecture easier to understand.

### 6. Do not split internal/manager yet

Do not perform a large package split just to improve directory aesthetics.

Keep the current package until one or more of these conditions becomes true:

- one subdomain grows to roughly 5-8 core files and continues expanding;
- PRs repeatedly change only one domain but require navigating many unrelated files;
- understanding the package requires excessive cross-file context;
- test fixtures or helpers begin leaking across unrelated domains.

When that point is reached, consider gradually splitting into `config`, `lifecycle`, `service`, and `schedule`.

---

## Execution Plan

Use four tickets. Keep behavior changes and refactors separate.

### Ticket 1: Make scheduler operations idempotent

Goal: make Linux/Darwin scheduling safe to call repeatedly.

Tasks:

- update `darwinPlatformScheduler.Set`;
- do not use plist existence as proof that the job is loaded;
- treat only explicit "job not loaded" bootout errors as idempotent;
- continue writing the plist and bootstrapping after stale plist detection;
- update `darwinPlatformScheduler.Stop`;
- allow stale plist removal even when the job is not loaded;
- return success when both plist and job are absent;
- preserve Linux/systemd behavior;
- add focused tests for:
  - Set: plist exists + job not loaded;
  - Set: plist exists + job loaded;
  - Stop: plist exists + job not loaded;
  - Stop: plist missing;
  - Stop: real bootout failure.

Acceptance criteria:

```text
Set can be called repeatedly
Stop can be called repeatedly
stale plist does not block recovery
real launchctl errors are still reported
```

Suggested commit:

```text
fix(schedule): make launchd operations idempotent
```

### Ticket 2: Close the ConfigApply state model

Goal: make status accurately describe the failed stage.

Tasks:

- add `ConfigApplyFailed`;
- change the default apply failure recording to `apply-failed`;
- write `validation-failed` only when validation actually fails;
- keep reload failures as `pending-reload`;
- keep success as `applied`;
- inspect CLI/TUI consumers for assumptions about the old state set;
- add tests for source, download, cancellation, mkdir, write, validation, rename, reload, and success paths.

Acceptance criteria:

Given any `Apply()` failure, the stored status should tell whether it was an apply failure, a validation failure, or a reload failure.

Suggested commit:

```text
fix(config): distinguish apply and validation failures
```

### Ticket 3: Simplify configPipeline.Apply()

Goal: reduce maintenance cost without changing behavior.

Tasks:

- extract only meaningful private stages;
- keep `Apply()` as the orchestration entry point;
- do not add new public interfaces;
- do not redesign the transaction;
- preserve cleanup and rollback behavior;
- keep state recording behavior introduced in Ticket 2 unchanged.

A desirable shape is:

```go
func (p *configPipeline) Apply(ctx context.Context) error {
    release, err := p.lock(ctx)
    if err != nil {
        return err
    }
    defer release()

    if err := p.refreshSubscription(ctx); err != nil {
        return p.failApply(err)
    }

    preview, err := p.buildPreview(ctx)
    if err != nil {
        return p.failApply(err)
    }

    staged, err := p.stageAndValidate(ctx, preview)
    if err != nil {
        return err
    }

    if err := p.commitConfig(staged); err != nil {
        return p.failApply(err)
    }

    return p.reloadAndRecord(ctx, preview)
}
```

This is illustrative, not mandatory. Avoid extracting trivial wrappers.

Acceptance criteria:

A reviewer should be able to understand the full apply flow from `Apply()` without reading low-level implementation details first.

Suggested commit:

```text
refactor(config): split apply pipeline stages
```

### Ticket 4: Clean up infrastructure naming and repository policy

Goal: improve long-term clarity without broad architecture changes.

Tasks:

- rename `GitHubReleases` to `ReleaseSource`;
- update constructor fields, mocks, tests, and references;
- do not split `OSSystem` as part of this ticket unless the change is naturally required;
- explicitly decide the repository tracking policy;
- replace the catch-all ignore rule if README/docs/CI/release metadata should be preserved.

Acceptance criteria:

- the release abstraction name reflects its actual responsibility;
- repository metadata cannot be accidentally removed by the ignore policy;
- no broad package split is introduced.

Suggested commits:

```text
refactor(lifecycle): rename release source abstraction
chore(repo): preserve repository infrastructure files
```

---

## Execution Order

Follow this order:

```text
1. scheduler idempotency
2. ConfigApply state semantics
3. Apply pipeline refactor
4. ReleaseSource naming + repository policy
```

Do not combine Ticket 2 and Ticket 3 into one commit.

Ticket 2 changes behavior.
Ticket 3 should be a behavior-preserving refactor.

Keeping them separate makes review and regression analysis much easier.

## Explicit Non-Goals

Do not include these in this maintenance pass:

- splitting `internal/manager` into multiple packages;
- introducing a dependency injection framework;
- adding repository/service/usecase/controller layers;
- creating interfaces for every pipeline stage;
- rewriting the full config pipeline;
- creating a single super-abstraction for Linux and Darwin service/scheduler behavior.

The target outcome is a smaller maintenance surface, clearer failure states, readable orchestration, and stable platform boundaries—not a larger architecture.
