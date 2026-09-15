## DAG

- `02-deterministic-subscription-source` → `03-transactional-subscription-update`
- `02-deterministic-subscription-source`, `03-transactional-subscription-update` → `05-linux-scheduled-subscription-update`
- `02-deterministic-subscription-source`, `03-transactional-subscription-update`, `05-linux-scheduled-subscription-update` → `06-darwin-scheduled-subscription-update`
- `04-verified-mihomo-installation` has no dependencies.
- `01-architecture-repair` is the parent/spec issue and is not scheduled as an implementation issue.

## Layers (Kahn L1..Ln)

- **L1**: `02-deterministic-subscription-source`, `04-verified-mihomo-installation`
- **L2**: `03-transactional-subscription-update`
- **L3**: `05-linux-scheduled-subscription-update`
- **L4**: `06-darwin-scheduled-subscription-update`

## Preflight

- `HEAD`: `91936ea4cdfbcdb1cb7efc02f4b5b89200e11316`
- `BASE_HEAD`: `91936ea4cdfbcdb1cb7efc02f4b5b89200e11316`
- Branch: `main`
- Worktree: 60 paths already modified or untracked before implementation; these are preserved and excluded from this feature's scope.
- Repository: Go 1.24 module; Go tool available as `go1.27.1-X:nodwarf5`.
- Code navigation: CodeGraph index and CLI available.
- Targeted test commands: `go test ./internal/manager`, `go test ./internal/cli`, `go test ./internal/scheduler`, plus `go test .` when root entrypoint/TUI behavior is affected.
- Acceptance/runtime path: Linux systemd is available and running; Darwin launchd is unavailable on this host, so Darwin acceptance remains unavailable here.
- Build/typecheck: affected `go test` commands compile the relevant packages; no separate release build is planned.
- Sensitive-scan tooling: `gitleaks` and `trufflehog` are unavailable; no secret files are read.

## Progress

| NN | Status | Commit | Review | Tests |
|---|---|---|---|---|
| 02 | resolved | `65363bf` | completed; no blocking findings | `go test ./internal/manager && go test .`
| 03 | resolved | `566fb04` | completed; no blocking findings | `go test . ./internal/cli ./internal/manager`
| 04 | resolved | `c0e882e` | completed; no blocking findings | `go test ./internal/manager && go test .`
| 05 | resolved | `HEAD` (implementation commit) | completed; no blocking findings | `go test . ./internal/cli ./internal/manager` + Arch Incus acceptance
| 06 | planned | — | — | — |
