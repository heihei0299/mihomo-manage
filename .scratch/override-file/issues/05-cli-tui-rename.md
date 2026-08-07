# 05 — CLI/TUI 更名与文案

**What to build:** CLI 命令与界面文案与覆写文件命名对齐：`config override edit` 取代 `config template edit`；旧 `template` 与 `config template` 命令输出报错并引导新命令；TUI 配置页显示覆写文件路径与编辑命令；help 文案更新，`template` 字样彻底退出界面。

**Blocked by:** 02（编辑目标为覆写文件）— 已于 dev 分支 3b89abb 完成

**Status:** resolved

- [x] `config override edit` 打开覆写文件编辑器（`TestConfigOverrideEditOpensOverrideFile`：编辑器收到 `OverrideFilePath`、`UpdateConfig` 被调用、退出码 0）
- [x] 旧 `template` / `config template` 命令输出报错并引导新命令（退出码非 0）（`TestConfigTemplateCommandDeprecated` / `TestLegacyTemplateCommandDeprecated`：stderr 引导 `config override edit`、退出码 1）
- [x] TUI 配置页显示覆写文件路径与 `override edit` 命令（`TestConfigViewOverrideTab`：含 override.yaml 路径与 `config override edit`，无 config-template 字样）
- [x] CLI help 无 `template` 字样残留（`TestUsageTextNoTemplate`；usageText/README 均无命令残留，`config rules` 弃用警告改指 override.yaml）

## 实施总结
- 提交：`d64036a` — `feat(cli): config override edit 取代 config template edit，template 字样退出界面`
- 提交：`e4638c3`（docs）— `docs: AT-12 命令更新为 config override edit 并更名覆写文件`
- 实现的 seams：
  - S1 `config override edit`：`handleConfigCommand`（提取自 main() 的 config 分发，与 handleSubscription 惯例一致，ctx 注入）→ `cliEditFile`（改返回退出码）→ 编辑 `OverrideFilePath`
  - S2 旧 `config template`：报错引导 `config override edit`，退出码 1，不触发编辑器/UpdateConfig
  - S3 顶层 `template`：`handleLegacyTemplate` → `deprecatedError` 引导，退出码 1
  - S4 TUI 配置页：tab 更名 `configTabOverride`，显示 `Override file: /opt/mihomo/etc/override.yaml`（引用 `OverrideFilePath` 常量）与 `config override edit`
  - S5 `usageText()`：`config override` 取代 `config template`，无命令名残留；保留 `MIHOMO_RELEASE_URL=<tmpl>` URL 模板占位符说明
- 验收标准：4/4 全绿（见上方 checkbox）
- 测试结果：全绿（go test ./... 四包共 130 个测试通过，含新增 6 个 main 包测试）
- typecheck：go vet ./... 与 go build ./... 通过
- 文档对齐：README 命令清单更新（template edit → config override edit）；docs/acceptance-tests.md AT-12 更新（命令与标题）；ADR-0007 为决策记录不改
- 遗留 / 后续建议：Code Review 修复了提取 handleConfigCommand 时丢失 config rules 分支的回归（补回并改指 override.yaml）；报错引导与迁移/占位符警告文案中提及旧命令/旧文件名属功能必需，保留
