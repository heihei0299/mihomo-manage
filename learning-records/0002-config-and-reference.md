# Learning Record 0002

**Date:** 2025-07-02
**Lessons:** 0002-config-management.html — 配置管理
**References:** glossary.html, cli-tui-reference.html

## Summary

学习了 mihomo-manager 的配置管道：三条输入（订阅数据 + 配置模板 + 分流规则）通过模板合并生成最终 config.yaml。掌握了 subscription set/update、template edit、rules edit、config preview、schedule 等命令的用法和流程。

## Key insights

- 模板中的 `{{subscription}}` 和 `{{routing_rules}}` 是两个独立的占位符，分别处理不同来源的数据
- `subscription update` 会自动备份当前配置（config.yaml.bak.<timestamp>），具备灾难恢复能力
- 定时刷新基于 time.Ticker 实现，支持任意 time.ParseDuration 格式
- 配置的生效不需要 restart，reload 即可热更新
- `config preview` 是排查问题的重要工具——先预览再更新

## Next steps

- 深入 TUI 各视图的交互细节
- 理解升级回滚机制的实现原理
- 了解日志查看（journalctl 集成）的不同使用场景
