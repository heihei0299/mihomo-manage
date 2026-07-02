Status: ready-for-agent

# 配置管理

## Parent

`.scratch/mihomo-manager/PRD.md`

## What to build

实现 config-pipeline 及相关配置管理功能。核心是模板渲染引擎和订阅管理。

### config-pipeline

接收 subscription-source（remote URL 或 local paste） + config-template（含 `{{subscription}}` 和 `{{routing_rules}}` 占位） + routing-rules 作为输入，输出最终 `config.yaml`。

模板渲染使用简单字符串替换（见 ADR-0001）。

### 功能清单

- **订阅管理**：添加 remote URL 订阅 / 粘贴本地配置内容。数据持久化到 manager state 中，以便自动刷新时复用。
- **模板编辑**：CLI `template edit` 调用 `$EDITOR` 打开模板文件；TUI 提供模板编辑界面。
- **路由规则编辑**：CLI `rules edit` 调用 `$EDITOR` 打开规则文件；TUI 提供规则编辑界面。
- **配置预览**：`config preview` 渲染当前模板+订阅+规则，输出到 stdout / TUI 文本视图。
- **订阅刷新**：`subscription update` 重新拉取订阅 → 重新渲染模板 → 备份当前 `config.yaml` → 写入新 `config.yaml` → reload 实例。支持 `--force` 跳过缓存。
- **定时刷新**：后台 goroutine/ticker 按用户配置的周期自动执行刷新流程。

### 自动备份

每次 `subscription update` 前将当前 `config.yaml` 复制为 `config.yaml.bak.<unix-timestamp>`。

### TUI 布局

本 slice 在 TUI 中新增配置管理标签页，包含：订阅列表、模板编辑区、规则编辑区、预览区。

## Acceptance criteria

- [ ] config-pipeline 正确处理三种输入：纯 remote、纯 local、混合（local 内容覆盖/合并 remote）
- [ ] 简单字符串替换正确注入 `{{subscription}}` 和 `{{routing_rules}}`
- [ ] CLI `subscription add`, `subscription update`, `template edit`, `rules edit`, `config preview` 全部可用
- [ ] TUI 配置管理标签页可按标签/键盘切换
- [ ] 自动备份在每次更新前正确创建 `.bak.<timestamp>` 文件
- [ ] 订阅刷新后自动执行 reload（依赖 Slice 4），或优雅提示用户手动 reload
- [ ] 定时刷新按配置周期执行
- [ ] Manager 模块 config-pipeline 测试覆盖：给定模板+订阅+规则→验证最终输出

## Blocked by

- `.scratch/mihomo-manager/issues/02-install-lifecycle.md`
