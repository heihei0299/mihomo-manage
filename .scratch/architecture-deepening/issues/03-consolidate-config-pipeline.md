Status: ready-for-agent

## Parent

Architecture deepening — [architecture review](/tmp/architecture-review-1783028521.html)

## What to build

将 config-pipeline 从 `manager` 结构体上拆离为独立的 `ConfigPipeline` 模块。

定义接口和选项：

```go
type ConfigPipeline interface {
    SetSubscriptionSource(ctx context.Context, source string) error
    SetRoutingRules(ctx context.Context, rules string) error
    Preview(ctx context.Context) (string, error)
    Apply(ctx context.Context) error
}

type ConfigPipelineOptions struct {
    OnReload  func(ctx context.Context) error
    Validator ConfigValidator
}

type ConfigValidator interface {
    Validate(ctx context.Context, configPath string) error
}
```

`ConfigPipeline` 实现涵盖：订阅数据获取（HTTP 或本地读取）→ 模板渲染（`renderConfig` 简单字符串替换）→ 备份当前配置 → 写入新配置 → 验证（通过 `ConfigValidator`）→ 回调 `OnReload`。所有文件 I/O、命令执行、HTTP 来自注入的 seam（拆分后的 `FileSystem` / `CommandRunner` / `GitHubReleases`，或暂用旧的 `System` 接口）。

已知 bug 修复：`config.go:136` 的 `exec.Command` 调用改为通过 `ConfigValidator` seam 进行，不再静默跳过验证。

`schedule.go` 中的定时触发改为调 `pipeline.Apply(ctx)`。

`lifecycle.go` 的 install 引导不受影响，继续直接写默认配置。

详见 ADR-0002。

## Acceptance criteria

- [ ] `ConfigPipeline` 接口和 `ConfigPipelineOptions` 定义
- [ ] `ConfigValidator` 接口 + production 实现（调 `mihomo -t`）+ test 假实现（永远返回 nil）
- [ ] `OnReload` 回调正确触发 `svcMgr.Reload`
- [ ] `manager` 上的 `SetSubscriptionSource`/`SetRoutingRules`/`PreviewConfig`/`UpdateConfig` 委托给 pipeline
- [ ] `schedule.go` 的 goroutine 调 `pipeline.Apply(ctx)` 而非 `m.UpdateConfig()`
- [ ] `config.go:136` 的 `exec.Command(mihomo -t)` 被 `ConfigValidator.Validate()` 替代
- [ ] config 验证在测试中可被 mock（之前静默跳过）
- [ ] 所有单元测试通过，验收测试通过
- [ ] 备份 + 回滚行为不变

## Blocked by

- Issue 01（拆分 System 接口）— 强烈建议在 #01 之后做，因为可用 `FileSystem`/`CommandRunner`/`GitHubReleases` seam

## References

- ADR-0002: `docs/adr/0002-config-pipeline-module.md`
