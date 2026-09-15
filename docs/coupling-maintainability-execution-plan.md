# Coupling maintainability execution plan

## Goal

Keep `internal/manager` maintainable without prematurely splitting Config, Lifecycle, or Service into new packages.

The plan reduces accidental cross-domain coupling first, measures real change blast radius, and introduces a compile-time package boundary only when the existing split triggers are repeatedly met.

## Scope

In scope:

- Config, Lifecycle, Service, and schedule orchestration ownership inside `internal/manager`.
- Shared contracts in `manager.go`, role interface files, `errors.go`, `paths.go`, and `defaults.go`.
- Review rules for cross-domain dependencies.
- Evidence required before extracting another package.

Out of scope:

- Rewriting the current manager architecture.
- Introducing repository/service/usecase/controller layers.
- Adding a DI framework.
- Splitting packages for symmetry or line count alone.
- Expanding `architecture_test.go` into symbol-level dependency analysis.

## Execution order

### Ticket 1 — Make cross-domain dependencies explicit in review

For every new dependency crossing Config, Lifecycle, Service, or Schedule ownership, require the change to answer:

1. Which area owns the state or invariant?
2. Why is the dependency needed?
3. Can the caller use an existing role interface instead of implementation internals?
4. Does the dependency introduce or strengthen a reverse dependency?

Rules:

- A same-domain helper stays in its owning area.
- A cross-domain call must use an existing stable role contract when one already represents the required capability.
- Do not introduce an interface solely to hide an otherwise unnecessary dependency.
- Do not move a helper to a shared file merely so two areas can reach it.

Acceptance criteria:

- Localized changes have an identifiable owner.
- New cross-domain dependencies are deliberate rather than incidental.
- No new generic shared/helper layer is introduced.

### Ticket 2 — Protect the shared-contract surface

Treat `manager.go`, role interface files, `errors.go`, `paths.go`, and `defaults.go` as constrained shared surfaces.

Before adding a symbol to a shared file, verify all of the following:

- It is genuinely needed by more than one ownership area, or it is a caller-facing contract.
- Its semantics are not owned by one specific domain.
- Moving it to shared does not bypass an existing ownership boundary.
- It does not add mutable cross-domain state.

If any condition fails, keep the symbol with its domain owner.

Acceptance criteria:

- Shared files do not become a dumping ground for convenience helpers.
- Domain-specific state remains with its domain.
- New caller-branchable errors continue to be centralized only when callers need to branch on them.

### Ticket 3 — Record change blast radius during normal development

Do not add runtime instrumentation or a permanent metrics subsystem. Record the following as part of review for each meaningful Config, Lifecycle, or Service change:

- owning area;
- production files changed in that area;
- production files changed outside that area;
- whether unrelated manager implementation had to be read to validate the change;
- whether a helper/type had to be promoted to shared;
- whether a new cross-domain dependency was introduced.

A change is a coupling signal when a localized domain change repeatedly requires unrelated ownership areas.

Acceptance criteria:

- Package-split decisions can cite recent concrete changes instead of intuition.
- Line count alone is never used as extraction evidence.

### Ticket 4 — Evaluate split triggers after repeated evidence

Evaluate an ownership area for package extraction only when at least two of the existing maintainability triggers persist across multiple changes.

Use this decision checklist:

- [ ] Three or more consecutive changes repeatedly touched four or more unrelated manager files.
- [ ] Searches for the area routinely return substantial unrelated implementation.
- [ ] The area has a stable contract and tests that rarely need package-wide internals.
- [ ] Review repeatedly requires unrelated manager context.
- [ ] Helpers/types are being promoted to shared mainly to cross ownership boundaries.

Decision rule:

- Fewer than two persistent signals: do not split.
- Two or more persistent signals: prepare a focused extraction proposal.
- Extract only one mature area at a time.

Acceptance criteria:

- Every package extraction has evidence tied to real maintenance cost.
- No Config/Lifecycle/Service package is extracted for symmetry.

### Ticket 5 — If extraction is justified, use the scheduler pattern

For the first domain that satisfies Ticket 4:

1. Identify the mature implementation mechanics that can move behind a small contract.
2. Keep orchestration, caller-facing semantics, and branchable errors in `internal/manager` unless ownership clearly belongs elsewhere.
3. Move only the mature implementation to a new `internal/<domain>` package.
4. Enforce one-way dependency from manager to the extracted package.
5. Move the extracted domain's focused tests with the implementation where appropriate.
6. Add only the smallest architecture guard needed to prevent reverse imports.
7. Do not extract another domain in the same change.

Acceptance criteria:

- Dependency direction is one-way.
- The extraction reduces the context needed for localized review.
- Public behavior and state semantics remain unchanged.
- No new architectural layer is introduced.

## Review checklist

For future changes, use this short checklist:

- [ ] Does every changed production file have one clear owner?
- [ ] Did the change introduce a cross-domain implementation dependency?
- [ ] Was anything promoted to shared only for convenience?
- [ ] Is state still owned and interpreted by one domain?
- [ ] Did a localized change require unrelated manager context?
- [ ] Does this change add evidence toward an existing package-split trigger?

## Stop conditions

Stop architecture work and return to normal feature development when:

- localized changes remain localized;
- no shared-surface pressure is accumulating;
- review does not require unrelated domain context;
- fewer than two package-split signals persist.

The default action is therefore **no further package split**. A new package is a response to demonstrated maintenance cost, not a target architecture milestone.
