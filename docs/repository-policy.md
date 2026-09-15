# Repository tracking policy

This repository tracks both the Go implementation and the files needed to
understand, build, release, and maintain it. That includes README files, the
license, CI and release workflows, documentation, and Markdown metadata such
as issue specifications.

`.gitignore` is an allow-by-default list: every ignored path is an explicitly
named generated or local-only artifact. Do not add a repository-wide catch-all
rule. When a new generated output is introduced, add its exact path or a
narrow pattern to `.gitignore`; source and repository infrastructure must stay
visible to Git so additions and removals appear in review.

## Required repository assets

These files are part of the project and must remain tracked:

- `README.md`
- `LICENSE`
- `.github/workflows/release.yml`
- `docs/`
- Markdown issue and project metadata under `.scratch/`

Ignored outputs currently include local binaries, build/package directories,
coverage output, the legacy generated template, and the local `04-issue/`
workspace. Everything else is intended to remain trackable.


## Maintenance source of truth

Architecture and maintenance boundaries are defined in
`docs/maintainability.md`. Review/remediation plans under `docs/` are execution
records; they must not become the only source of truth for current runtime
behavior or package ownership.
