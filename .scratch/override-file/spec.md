# 覆写文件（override-file）取代模板：本地优先的合并语义

Status: ready-for-agent

## Problem Statement

订阅内容（被拉取的文件）作为配置 base，本地定制只能「补充不能覆盖」：订阅里已有的字段（如 `port`、`dns.enable`）无法被本地修改，手动编辑最终 `config.yaml` 的修改也会在订阅刷新时被重新生成的内容覆盖丢失。

## Solution

将 config-template 升级并改名为本地覆写文件（override-file），合并语义改为本地优先：任意字段可覆盖订阅已有值，数组默认追加、标 `!replace` 时整体替换；`config.yaml` 定位为纯生成物，本地定制一律写入覆写文件，`config adopt` 将既有手动修改迁移进覆写文件；配置校验在生成时（失败回滚）与 start 前（失败拒绝启动）双重执行。

## User Stories

1. As a user, I want to override any field from the subscription (e.g. `port`, `dns.enable`), so that I am not bound by the provider's config
2. As a user, I want to supplement fields missing from the subscription, so that I can add `dns`/`tun`/`experimental` sections
3. As a user, I want arrays (`proxies`, `proxy-groups`, `rules`, `proxy-providers`, `rule-providers`) to append to the subscription arrays by default, so that new nodes from subscription updates are kept automatically
4. As a user, I want to mark an array with `!replace` to replace the subscription array entirely, so that I have full control over specific list configs
5. As a user, I want the old config-template to migrate to the override file automatically, so that upgrading is seamless
6. As a user, I want `config adopt` to extract my manual edits from the current config.yaml into the override file, so that my changes survive subscription refreshes
7. As a user, I want `config adopt` to be repeatable (idempotent), so that accidental manual edits can be recovered at any time
8. As a user, I want `config adopt` to show a diff summary and require confirmation when the difference is large, so that subscription changes are not mistakenly absorbed
9. As a user, I want config validation before `start` to refuse startup on failure, so that a manually broken config cannot take down the service
10. As a user, I want `config override edit` instead of `template edit`, so that the CLI matches the file name
11. As a user, I want installation to generate a commented default override file, so that I know what I can write there
12. As a user, I want a pure-subscription setup by deleting the override file, so that simple scenarios need no local config
13. As a user, I want generation-time validation with automatic rollback on failure, so that a bad subscription update does not break the running config
14. As a user, I want config preview to reflect the new merge semantics, so that I can verify the result before applying

## Implementation Decisions

- **ConfigManager 接口扩展**：新增 `ValidateConfig(ctx) error`（复用 `ConfigValidator` seam）与 `AdoptConfig(ctx, force bool) (AdoptReport, error)`；`UpdateConfig` 内部执行新合并语义。此为唯一行为 seam，不新增运行时 seam。
- **合并语义**：覆写文件为 overlay，订阅数据为 base——同名标量/映射：覆写文件覆盖订阅；订阅缺失字段：覆写文件补充；五个数组字段（`proxies`、`proxy-groups`、`rules`、`proxy-providers`、`rule-providers`）：默认追加，标 `!replace`（YAML 自定义 tag，yaml.v3 解析）时整体替换订阅数组。
- **adopt 语义**：对比当前 `config.yaml` 与「订阅数据+覆写文件」渲染结果的顶层字段差异。标量/映射字段差异写入覆写文件（以当前 `config.yaml` 为准）；数组字段差异仅在报告中列出、不吸收。候选写入字段 ≥ 5 个判定为大差异，需用户确认（`--force` 跳过确认）。adopt 幂等：重复执行无差异时报告 no changes。
- **自动迁移**：pipeline 初始化时检测旧 config-template 存在且覆写文件不存在 → 自动改名并提示一次；此后旧名不再识别。
- **start 前校验**：CLI/TUI 的 start 与 restart 命令执行前调用 `ValidateConfig`，失败拒绝启动并输出校验错误；install/upgrade 流程内部的 start 不新增校验。
- **命名**：覆写文件路径常量更名为 OverrideFilePath；安装引导写入带注释的默认覆写文件（含五个数组字段与 `!replace` 用法示例），内容可缺省（文件不存在 = 纯订阅生效）。
- **CLI**：`config template edit` 更名为 `config override edit`；新增 `config adopt [--force]`；旧 `template` 命令与 `config template` 输出报错并引导新命令；TUI 配置页文案同步更新。
- 生成时校验与回滚、备份策略（`config.yaml.bak.<timestamp>`）维持现有行为不变。

## Testing Decisions

- **mergeConfig 纯函数表驱动测试**：覆盖新语义全部边角——同名标量/映射覆盖、嵌套映射覆盖、缺失补充、五个数组追加、`!replace` 整体替换、非法 YAML、非 map 覆写。现有「overlay 不覆盖 base」断言全部反转更新。
- **ConfigManager 行为测试**（mock `FileSystem`/`ConfigValidator`）：旧模板自动迁移（含旧文件不存在、新文件已存在两种路径）；adopt 幂等（二次执行 no changes）；adopt 大差异需确认（≥5 字段）与数组差异不吸收；`ValidateConfig` 失败传播。
- **Handler 编排测试**：start/restart 在校验失败时拒绝启动；`config adopt` 命令输出报告。
- Prior art：`merge_test.go`（纯函数表驱动）、`config_update_test.go`（pipeline 行为）、`handler_test.go`（编排）。

## Out of Scope

- 数组按 name 标识符合并（如 proxy-groups 按 name 匹配合并参数）
- 多覆写文件 / 分层覆写机制
- adopt 自动吸收数组字段差异
- install/upgrade 流程内部 start 的配置校验
- 覆写文件的语法校验与编辑器的语法高亮
- 订阅数据文件的手动编辑（仍通过 subscription-source 管理）

## Further Notes

- ADR-0007 已落盘（`docs/adr/0007-override-file-semantics.md`），演进自 ADR-0002：保留 YAML 深层合并机制与数组追加默认，推翻「模板不覆盖订阅字段」规则
- CONTEXT.md 术语已同步（override-file / subscription-data 入册，config-template 列为 Avoid）
- 发布后实现走 `/tdd-implement` 按 issue 拆分
