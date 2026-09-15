Status: resolved

## Parent

[architecture-repair](./01-architecture-repair.md)

## What to build

Make remote mihomo installation and upgrade fail closed when the downloaded release cannot be verified. The existing CLI and TUI installation flows must obtain release checksum metadata, verify the temporary artifact before decompression and deployment, support checksum metadata for custom mirrors, and preserve explicit local installation without remote lookup. Download and verification cancellation and errors must be visible to the caller.

## Acceptance criteria

- [x] A remote release with a matching checksum proceeds through the existing installation or upgrade flow.
- [x] A missing checksum, malformed checksum, or mismatched checksum prevents decompression, deployment, and service replacement.
- [x] A custom release mirror must provide corresponding checksum metadata before remote installation is allowed.
- [x] An explicitly supplied local binary remains installable without remote checksum lookup.
- [x] Temporary download artifacts are cleaned up after success and failure where safe.
- [x] CLI and TUI report verification failures as failed installation or upgrade operations.
- [x] Download cancellation reaches the underlying request and does not leave a successful-looking installation.
- [x] Tests cover matching, missing, malformed, mismatched, custom-mirror, local-install, cleanup, and cancellation behavior.

## Implementation summary

Added release checksum retrieval for GitHub and custom mirrors, fail-closed artifact verification before decompression, local-install bypass, signal-aware CLI/TUI contexts, temporary-artifact cleanup, and state-preserving lifecycle rollback.

## Verification

- `go test ./internal/manager`
- `go test .`
- Scoped code review completed with no remaining blocking or correctness findings after delta rechecks.

## Blocked by

None - can start immediately
