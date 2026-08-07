# 02 — 覆写文件命名迁移：override.yaml 取代 config-template.yaml

**What to build:** config-template 更名为本地覆写文件并自动迁移：pipeline 初始化时检测旧文件存在且新文件不存在 → 自动改名并提示一次，此后旧名不再识别。安装引导生成带注释的默认覆写文件（含五个数组字段与 `!replace` 用法示例）；覆写文件可缺省（删除它 = 纯订阅直接生效）。

**Blocked by:** None — 可立即开始

**Status:** resolved

- [x] 路径常量更新为覆写文件；Preview/Apply 从覆写文件读取
- [x] 迁移逻辑测试：旧文件存在+新文件不存在 → 改名成功并提示一次；新文件已存在 → 不改动；两者均不存在 → 无操作
- [x] 安装 bootstrap 写入带注释的默认覆写文件（含 `!replace` 用法示例）
- [x] 覆写文件不存在时管线正常工作（纯订阅生效）
- [x] 旧 config-template.yaml 不再被识别（存在也不读取）

## 实施总结
- 提交：`3b89abb` — `feat(core): 覆写文件命名迁移 override.yaml 取代 config-template.yaml`
- 实现的 seams：`ConfigManager` 构造（`migrateLegacyTemplate` 自动迁移）、`Preview`/`Apply`（读取/容忍缺失）、`lifecycle` bootstrap（默认覆写文件）、CLI 编辑路径（main.go）、acceptance 断言
- 验收标准：5/5 全绿（TestPipelineMigratesLegacyTemplate / MigrationSkipsWhenOverrideExists / MigrationNoopWhenNothingExists / TestPipelinePureSubscriptionWithoutOverride / TestLifecycleInstallWritesDefaultOverride；迁移后旧路径内容随 rename 保留）
- 测试结果：全绿（go test ./... 三包全过；pipeline/lifecycle/merge 25 项 PASS）
- typecheck：go vet ./... 与 go build ./... 通过
- 文档对齐：docs/acceptance-tests.md 安装断言与 AT-12 编辑路径同步为 override.yaml
- 遗留 / 后续建议：CLI 命令名（template edit → override edit）与 TUI 文案属 05 票范围；占位符模板警告文案保留旧路径描述（迁移期提示）
