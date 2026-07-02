Status: ready-for-agent

# TUI 配置管理标签页

## Parent

`.scratch/mihomo-manager/PRD.md`

## What to build

在 TUI 中新增配置管理标签页，补齐当前 TUI 缺失的功能。Bubble Tea 当前只有仪表盘视图，需要扩展为多标签页布局。

新标签页包含：

- **订阅视图**：显示当前订阅源（URL 或 local），上次刷新时间，手动刷新按钮
- **模板编辑区**：直接编辑 config-template.yaml 的内容（文本编辑区/调用 `$EDITOR`）
- **规则编辑区**：直接编辑 rules.txt 的内容
- **配置预览区**：显示 `config preview` 的渲染结果（只读文本视图）

标签切换通过键盘 `<Tab>` 或方向键实现。

此 Slice 依赖 Slice 07（CLI 模板/规则编辑）先做好——TUI 的编辑功能在底层复用相同的文件写入逻辑。

## Acceptance criteria

- [ ] TUI 支持多标签页布局，Tab 键切换
- [ ] 订阅视图显示当前源和刷新时间
- [ ] 模板编辑区可编辑内容并保存
- [ ] 规则编辑区可编辑内容并保存
- [ ] 配置预览区显示当前渲染结果
- [ ] 编辑保存后自动刷新预览

## Blocked by

- `.scratch/mihomo-manager/issues/07-cli-template-rules-edit.md`
