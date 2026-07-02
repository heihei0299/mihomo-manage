Status: ready-for-agent

## Parent

Architecture deepening — [architecture review](/tmp/architecture-review-1783028521.html)

## What to build

拆分 `System` 接口成为三个独立的 seam：`FileSystem`（7 文件操作方法）、`CommandRunner`（2 命令执行方法）、`GitHubReleases`（3 HTTP/GitHub 方法）。`OSSystem` 改为同时实现三个接口。`manager` 结构体用三个字段替代当前的一个。

`mockSystem` 拆分为 `fakeFileSystem`、`fakeCmdRunner`、`fakeGitHubReleases` 三个独立 mock。每个测试只 mock 需要的 seam。

`main.go` 的 wiring（`manager.New(sys, svcMgr)`）改为传入三个 seam。

无行为变更，纯重构。

详见 ADR-0003。

## Acceptance criteria

- [ ] `FileSystem`、`CommandRunner`、`GitHubReleases` 三个接口在 `system.go` 中定义
- [ ] `OSSystem` 同时实现三个接口，逻辑不变
- [ ] `manager` 结构体用 `fs FileSystem`、`cmd CommandRunner`、`gh GitHubReleases` 替代 `sys System`
- [ ] `New()` 构造函数签名更新
- [ ] `main.go` 传入三个实参
- [ ] `mockSystem` 拆分为三个独立的 mock，每个 mock 只实现对应接口
- [ ] 所有测试通过（单元测试 + 验收测试）
- [ ] mock 测试中不再出现不相关的 stubbed 方法

## Blocked by

None — can start immediately.

## References

- ADR-0003: `docs/adr/0003-split-system-interface.md`
