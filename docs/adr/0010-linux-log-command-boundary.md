# ADR-0010: Keep log lookup Linux-only

- Status: Accepted
- Date: 2026-09-19

## Decision

The `logs` CLI command remains in the composition root and explicitly invokes
Linux `journalctl`. Non-Linux platforms return `UnsupportedPlatformError`
with feature `logs` instead of attempting a platform-specific fallback.

## Consequences

Linux retains the existing output and exit-code behavior. Darwin and other
platforms receive a branchable unsupported-platform error; a future native log
adapter requires an explicit platform implementation and tests.
