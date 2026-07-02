Status: ready-for-agent

# CLI 模板/规则编辑

## Parent

`.scratch/mihomo-manager/PRD.md`

## What to build

实现 `mihomo-manager template edit` 和 `mihomo-manager rules edit` 两个 CLI 子命令。

- `template edit` 调用 `$EDITOR` 打开 `/opt/mihomo/etc/config-template.yaml`，保存后自动重新渲染配置（调用 `UpdateConfig`）
- `rules edit` 调用 `$EDITOR` 打开 `/opt/mihomo/etc/rules.txt`，保存后自动重新渲染配置（调用 `UpdateConfig`）

`Manager` 接口上已有的 `SetRoutingRules` 方法从未被 CLI 调用过——本次将其接入 CLI 路由。对于模板文件，直接修改文件系统路径（`FileOperator.WriteFile`）比通过 Manager 接口更直接，因此 Manager 无需新增方法。

## Acceptance criteria

- [ ] `mihomo-manager template edit` 打开 `$EDITOR`，编辑保存后自动重新生成配置
- [ ] `mihomo-manager rules edit` 打开 `$EDITOR`，编辑保存后自动重新生成配置
- [ ] 无 `$EDITOR` 环境变量时给出明确错误提示
- [ ] 文件不存在时自动创建空文件
- [ ] Manager 模块有测试覆盖保存→re-render 路径

## Blocked by

None - can start immediately
