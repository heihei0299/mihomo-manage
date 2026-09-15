# 05: 收敛文档导航与 source of truth

**What to build:** 让维护者能够从单页了解 ADR 的当前状态，让已完成的整改计划明确作为历史记录，并让 README 聚焦常用用户入口而不复制会漂移的完整 CLI help。

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] 增加唯一 ADR 索引，列出所有现有 ADR 的编号、主题和 Accepted/Superseded 等状态。
- [x] 已完成的 remediation plan 标记为 completed/historical execution record。
- [x] remediation plan 明确指向当前维护规则，不继续追加新的 runtime 设计要求。
- [x] README 保留 install、status、subscription、config、upgrade、uninstall 等常用命令指导。
- [x] README 不再复制完整 CLI help，并指向可执行程序自身的 help 输出。
- [x] 当前维护边界的 source of truth 仍是维护规则文档。
- [x] 不新增 README/doc generator 或复杂 documentation CI。

## 实施总结

- 新增 ADR 单页索引，覆盖 0001–0008 及其当前/历史状态。
- 将 incremental review remediation plan 标记为 completed historical execution record，并指向维护规则文档。
- README 删除完整 help 副本，保留常用命令与 `mihomo-manager --help` 入口。
- 验证：ADR 链接、README help 去重和文档 diff 静态检查通过。
- Review：Standards 与 Spec review 通过。
