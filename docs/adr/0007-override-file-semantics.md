# 覆写文件取代模板：本地优先的合并语义

config-template.yaml 原为「只补充不覆盖」的 overlay（ADR-0002），订阅刷新后用户无法覆盖订阅字段，手动编辑最终 config.yaml 的修改也会被重新生成覆盖丢失。决定将 config-template.yaml 改名为 override.yaml（override-file），合并语义升级为本地优先：任意字段可覆盖订阅已有值，数组默认追加、标 `!replace` 时整体替换；config.yaml 定位为纯生成物，本地定制一律写入覆写文件，`config adopt` 命令从既有 config.yaml 提取差异完成一次性迁移；配置校验保留生成时（`mihomo -t` 失败回滚）并增加 start 前校验。

## Status

accepted（演进自 ADR-0002：保留 YAML 深层合并机制与数组追加默认，推翻「模板不覆盖订阅字段」规则）

## Considered Options

- 独立 override 文件与模板并存分工 — 补充与覆盖分离，但两文件职责重叠，用户需同时理解两种语义
- 白名单覆盖 — 仅允许特定字段覆盖，维护成本高、边界难解释
- 数组按 name 标识符合并 — 语义强大但复杂，rules 等无标识符的数组不适用
- 覆写文件数组默认整体替换 — 与现有追加行为不兼容，破坏订阅新增节点的体验

## Consequences

- 检测到旧 config-template.yaml 时自动迁移为 override.yaml，旧名不再识别
- CLI `template edit` 更名为 `override edit`；新增 `config adopt`
- mergeConfig 增加任意字段覆盖语义与 `!replace` tag 解析
- start 生命周期增加配置校验步骤（校验失败拒绝启动）
- 安装引导生成带注释的默认 override.yaml（可缺省）
