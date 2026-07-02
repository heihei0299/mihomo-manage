Status: ready-for-agent

## Parent

Architecture deepening — [architecture review](/tmp/architecture-review-1783028521.html)

## What to build

将 `main.go` 中的 11 个 CLI 处理函数（`cliStatus`、`cliInstall` 等）提取到 `internal/cli` 包。定义 `Handler` 结构体：

```go
package cli

type Handler struct {
    mgr    manager.Manager
    stdout io.Writer
    stderr io.Writer
}

func New(mgr manager.Manager, stdout, stderr io.Writer) *Handler
```

每个 CLI 命令成为 `Handler` 的一个方法，返回退出码 `int`。`main.go` 保留参数解析和调度（将解析后的命令映射为 `Handler` 方法调用 + `os.Exit(code)`）。

`quietPrintf`/`quietPrintln` 被 writer seam 替代。

新增 `internal/cli/handler_test.go`：注入 `bytes.Buffer` 作为 stdout/stderr，断言输出和退出码。

详见 ADR-0004。

## Acceptance criteria

- [ ] `internal/cli` 包存在，包含 `Handler` 结构体和 `New()` 构造函数
- [ ] 11 个 CLI 命令作为 `Handler` 的方法实现，均返回 `int` 退出码
- [ ] 所有输出通过 `h.stdout`/`h.stderr`（`io.Writer`），不直接写 `os.Stdout`
- [ ] `main.go` 保留参数解析 + 赋值到命令 + `os.Exit(code)` 三件套
- [ ] `quiet` 全局变量和 `quietPrintf`/`quietPrintln` 函数移除
- [ ] 至少 3 个 CLI handler 有单元测试（如 `Status`、`Start`、`PreviewConfig`）
- [ ] 验收测试全部通过（不修改验收测试本身）
- [ ] 现有 CLI 输出格式完全不变

## Blocked by

None — can start immediately.

## References

- ADR-0004: `docs/adr/0004-cli-module-extraction.md`
