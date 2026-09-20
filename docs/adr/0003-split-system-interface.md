# Split System interface into three seams

## Context

The `System` interface (system.go:15-29) exposes 12 methods covering three distinct responsibilities: file system operations (7 methods), command execution (2 methods), and HTTP/network operations (3 methods). Implementation (`OSSystem`) is thin — each method is a 1-3 line stdlib wrapper.

The single interface is shallow: the interface surface is nearly as complex as the implementation. Tests must mock all 12 methods even when a test exercises only file I/O.

## Decision

Split `System` into three separate interfaces, each behind its own seam:

```go
type FileSystem interface {
    FileExists(path string) (bool, error)
    ReadFile(path string) ([]byte, error)
    WriteFile(path string, data []byte, perm uint32) error
    RemoveAll(path string) error
    Rename(oldPath, newPath string) error
    MkdirAll(path string, perm uint32) error
    Chmod(path string, perm uint32) error
}

type CommandRunner interface {
    RunCommand(ctx context.Context, name string, args ...string) (string, error)
    RunCommandIgnoreExit(ctx context.Context, name string, args ...string) (string, error)
}

type ReleaseSource interface {
    Download(ctx context.Context, url, dest string) error
    ExpectedChecksum(ctx context.Context, owner, repo, version, assetName string) (string, error)
    ListVersions(ctx context.Context, owner, repo string, limit int) ([]VersionInfo, error)
    LatestVersion(ctx context.Context, owner, repo string) (string, error)
}
```

### Rationale

- `FileExists` returns `false, nil` only for `os.ErrNotExist`; other stat errors remain observable
- `RemoveAll` makes the recursive deletion semantics explicit at every call site
- Each seam hides a genuine OS/network interaction behind a small interface
- Tests mock only what they need (e.g., config tests mock `FileSystem`, lifecycle tests mock `FileSystem` + `CommandRunner`)
- The deletion test passes for each seam independently
- `Download` and checksum retrieval are grouped with release operations; the source may be GitHub or a configured mirror

## Consequences

- `manager` struct gains three fields instead of one
- `mockSystem` splits into `fakeFileSystem`, `fakeCmdRunner`, `fakeReleaseSource`
- Test surface narrows per test: each mock implements 2-7 methods instead of 12
- Existing tests need refactoring to pass the right mock combinations
- The filesystem contract is intentionally stricter than the former boolean/delete pair: callers must handle stat errors and explicitly acknowledge recursive cleanup
