# 05: 统一 CLI/TUI 外部命令行为

**What to build:** 让 CLI 和 TUI 对 `$EDITOR` 的解析和参数传递保持一致，支持带参数的编辑器配置；编辑器失败、取消或空输入不会触发配置更新。日志命令明确限定 Linux 能力，非 Linux 平台返回可识别的 unsupported 错误。现有 CLI 输出和退出码保持兼容。

**Blocked by:** None (can start immediately).

**Status:** claimed

- [ ] CLI 和 TUI 使用同一个 editor command 构造 implementation。
- [ ] `EDITOR` 包含可执行文件参数时，两个入口产生相同的命令和参数序列。
- [ ] 编辑器启动失败、读取结果失败、用户取消或结果为空时，不调用配置更新。
- [ ] CLI 和 TUI 在订阅编辑后的配置更新错误上保持一致的用户可见诊断。
- [ ] Linux 日志能力继续可用，并明确使用 Linux 日志实现。
- [ ] 非 Linux 平台执行日志命令时返回明确的 unsupported 错误，而不是尝试执行不存在的系统命令。
- [ ] 通过现有 main、TUI 和 CLI handler 测试验证行为，不引入通用 process framework。
- [ ] 不改变其他 CLI 命令的现有输出格式和退出码契约。
