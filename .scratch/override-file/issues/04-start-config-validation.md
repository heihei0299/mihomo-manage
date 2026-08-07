# 04 — start/restart 前配置校验

**What to build:** 新增配置校验能力（复用既有校验 seam）：CLI/TUI 的 start 与 restart 命令执行前先校验配置，失败则拒绝启动并输出校验错误。install/upgrade 流程内部的 start 不新增校验。

**Blocked by:** None — 可立即开始

**Status:** ready-for-agent

- [ ] 校验方法测试：校验失败错误传播；成功时无副作用
- [ ] start 命令校验失败 → 拒绝启动、输出错误、退出码非 0
- [ ] restart 命令同样校验
- [ ] 校验通过时启动流程不受影响
