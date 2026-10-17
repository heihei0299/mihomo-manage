# ADR-0012: Restyle the TUI with lipgloss

- Status: Accepted
- Date: 2026-10-17

## Decision

The TUI presentation layer uses `github.com/charmbracelet/lipgloss`, promoted
from an indirect dependency to a direct one. The module was already in the
dependency tree via bubbletea, so this adds no new module download and leaves
go.sum unchanged.

Dynamic elements stay first-party: the action spinner (80ms braille frames),
the elapsed-time display, and Preview-tab scrolling are implemented with
`tea.Tick` and window slicing instead of adding `charmbracelet/bubbles`.

The dashboard renders as a rounded-border card sized to its content, with
semantic adaptive colors (running green, stopped yellow, not installed dim,
errors red) and a borderless single-column fallback on terminals narrower than
the card. The TUI runs on the alternate screen buffer (`tea.WithAltScreen`)
and handles `tea.WindowSizeMsg` so the Preview tab windows its content to the
terminal height.

## Considered Options

- Hand-rolled ANSI escapes — no direct dependency, but re-implements color
  profile downgrade, NO_COLOR handling, and ANSI-aware width measurement
- `charmbracelet/bubbles` for spinner and viewport — more features than needed
  plus a genuinely new module, against the project's dependency-restraint
  stance
- Keep the hand-drawn fixed-width box — misaligns on long values and offers no
  color semantics

## Consequences

- lipgloss moves into the direct require block of go.mod; go.sum is unchanged
- Rendering degrades to plain text on non-terminals and with NO_COLOR, so view
  tests keep asserting the same user-visible substrings
- New helpers (`formatDuration`, `windowLines`, `joinColumns`) carry focused
  unit tests; view changes must keep the documented key semantics intact
- `docs/acceptance-tests.md` AT-19 documents the alt-screen startup, spinner,
  and Preview scrolling behavior
