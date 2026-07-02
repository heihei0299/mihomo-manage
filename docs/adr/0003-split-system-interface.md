# Split System interface into three seams

## Context

The `System` interface (system.go:15-29) exposes 12 methods covering three distinct responsibilities: file system operations (7 methods), command execution (2 methods), and HTTP/network operations (3 methods). Implementation (`OSSystem`) is thin — each method is a 1-3 line stdlib wrapper.

The single interface is shallow: the interface surface is nearly as complex as the implementation. Tests must mock all 12 methods even when a test exercises only file I/O.

## Decision

Split `System` into three separate interfaces, each behind its own seam:

```go
type FileSystem interface {
    FileExists(path string) bool
    ReadFile(path string) ([]byte, error)
    WriteFile(path string, data []byte, perm uint32) error
    Remove(path string) error
    Rename(oldPath, newPath string) error
    MkdirAll(path string, perm uint32) error
    Chmod(path string, perm uint32) error
}

type CommandRunner interface {
    RunCommand(name string, args ...string) (string, error)
    RunCommandIgnoreExit(name string, args ...string) (string, error)
}

type GitHubReleases interface {
    Download(ctx context.Context, url, dest string) error
    ListVersions(ctx context.Context, owner, repo string, limit int) ([]VersionInfo, error)
    LatestVersion(ctx context.Context, owner, repo string) (string, error)
}
```

### Rationale

- Each seam hides a genuine OS/network interaction behind a small interface
- Tests mock only what they need (e.g., config tests mock `FileSystem`, lifecycle tests mock `FileSystem` + `CommandRunner`)
- The deletion test passes for each seam independently
- `Download` is grouped with GitHub operations because the only consumer is binary download

## Consequences

- `manager` struct gains three fields instead of one
- `mockSystem` splits into `fakeFileSystem`, `fakeCmdRunner`, `fakeGitHubReleases`
- Test surface narrows per test: each mock implements 2-7 methods instead of 12
- Existing tests need refactoring to pass the right mock combinations
