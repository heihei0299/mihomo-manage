Status: ready-for-agent

# 配置深合并 — 模板作为订阅文件的扩展

## Problem Statement

用户的 `config-template.yaml` 是完整的 mihomo 配置（不含 `{{subscription}}` 占位符），订阅 URL 返回的也是完整配置。当前 `Preview` 在模板不含占位符时直接返回订阅数据，模板的自定义内容（proxy-groups、rules、TUN/DNS 设置等）被完全丢弃。

用户期望：订阅作为 BASE，模板作为 OVERLAY，两者 YAML 深合并——模板的 proxy-groups、rules 叠加在订阅配置之上，且模板的列表项优先匹配。

## Solution

在 `configPipeline` 中新增 YAML 深合并路径。当模板不含 `{{subscription}}` 占位符且订阅数据存在时，将订阅解析为 base、模板解析为 overlay，执行深合并后输出。

合并规则：
- **maps**：递归合并，overlay（模板）的 key 覆盖 base（订阅）的同名 key，base 独有的 key 保留
- **lists**：overlay（模板）的项前置到 base（订阅）的项之前，确保模板定义的路由规则优先匹配
- **scalars**：overlay（模板）的值覆盖 base（订阅）的值
- 类型不一致时以 overlay（模板）为准

模板含 `{{subscription}}`/`{{routing_rules}}` 占位符时继续走现有字符串替换路径，不受影响。

## User Stories

1. As a mihomo-manager user, I want my `config-template.yaml` to extend rather than replace the subscription config, so that I can maintain custom proxy-groups, rules, and DNS/TUN settings on top of the provider's config.
2. As a mihomo-manager user, I want my template's rules to be matched before the subscription's rules, so that my custom routing takes priority.
3. As a mihomo-manager user, I want `mihomo-manager -c` to show the correctly merged result, so that I can preview the final config before applying.
4. As a mihomo-manager user, I want `mihomo-manager -u` to write the merged result to `config.yaml`, so that the running instance uses the combined configuration.
5. As a developer, I want the deep merge to handle nested maps correctly (e.g., `dns.nameserver-policy`), so that complex multi-level YAML structures merge predictably.
6. As a developer, I want the merge to produce valid YAML with proper indentation, so that `mihomo -t` validation always passes.
7. As a developer, I want the existing `{{subscription}}` placeholder path to remain unchanged, so that users who rely on string substitution are not affected.
8. As a mihomo-manager user, I want the merge to preserve all subscription data that my template doesn't override, so that I don't lose provider-specific settings.

## Implementation Decisions

### 新增模块

- **`mergeYAML(base, overlay []byte) ([]byte, error)`** — YAML 深合并纯函数，位于 `internal/manager/merge.go`
  - 订阅 YAML → `map[string]any`（base）
  - 模板 YAML → `map[string]any`（overlay）
  - `deepMerge(base, overlay)` 递归合并
  - 序列化为 YAML bytes
- **依赖**：`gopkg.in/yaml.v3`（mihomo 生态标准 YAML 库）

### 修改模块

- **`configPipeline.Preview()`**：模板不含占位符且订阅数据存在时，调用 `mergeYAML(subData, tmplData)` 替代直接返回 `subStr`
- **`configPipeline.Apply()`**：与 `Preview` 一致（`Apply` 调用 `Preview`，自动继承变更）

### 预览不影响验证

`Apply` 在 `Preview` 输出后走 `mihomo -t` 验证，合并结果不合法时会自动回滚。深合并产生的配置同样经过此验证。

## Testing Decisions

### 什么构成好的测试

- 测试外部行为而非实现：验证 `PreviewConfig()` 输出文本包含预期内容
- 订阅数据 + 模板 → 合并结果可断言
- 覆盖列表前置、map 覆盖、嵌套结构、类型不匹配等边角情况

### Seams

| Seam | 级别 | 测试方式 |
|------|------|---------|
| `mergeYAML` | 单元 | table-driven，base+overlay 各种组合 |
| `configPipeline.Preview()` 通过 `fakeFileSystem` | 集成 | 完整路径：订阅文件 + 模板文件 → 合并输出 |

### 先例

- 现有 `TestUpdateConfigHappyPath`、`TestPreviewConfig*` 系列测试使用同一套 `fakeFileSystem` + `fakeGitHubReleases` mock
- 新测试复用 `fakeFileSystem`，沿用 `written` map 设置文件内容的模式

## Out of Scope

- TUI 预览视图的修改（TUI 直接调用 `PreviewConfig`，自动受益）
- 非 YAML 格式的订阅数据（mihomo 配置始终是 YAML）
- CLI 界面改动（`-c` 输出不变，只是内容变成合并结果）
- macOS launchd 支持（Linux first）

## Further Notes

- 模板含 `{{subscription}}` 时保持字符串替换路径不变——这是给轻量级使用场景的路径
- 深合并路径的触发条件是：模板不含 `{{subscription}}` 且订阅数据非空
- 如果将来用户想在模板中使用占位符 + 深合并混合模式，需要重新设计——当前两个路径互斥
