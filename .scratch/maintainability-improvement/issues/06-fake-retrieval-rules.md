# 06: 加固 shared fake 与代码检索边界

**What to build:** 让测试基础设施和代码检索都遵循领域边界：shared fake 只提供多数测试共用的行为，单测试故障由本地 specialized fake 表达，config/schedule/lifecycle 任务各自有明确的首选读取范围。

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] shared fake 只保留多数测试都会使用的通用行为。
- [x] 明显单用途的阻塞、失败或特殊行为由对应测试组内的 specialized fake 提供。
- [x] 不创建新的 God Fake，不引入通用测试 fixture/framework。
- [x] config 任务优先读取 config implementation、merge/adopt/lock 相关代码和对应行为测试；只有明确依赖时才扩展到 service、lifecycle 或 scheduler。
- [x] schedule 任务优先读取 scheduler implementation 和对应测试，不因同 package 自动读取 config/lifecycle 实现。
- [x] lifecycle 任务优先读取 lifecycle implementation、service contract 和对应测试。
- [x] 维护规则中明确记录上述 config/schedule/lifecycle 的首选检索边界。
- [x] 本票不进行大规模代码重写或 package extraction。

## 实施总结

- shared `fakeFileSystem` 仅保留文件存在、读写、删除和重命名记录等通用行为。
- staging mkdir/write、backup write、rename、source marker write、读文件失败、staging cleanup 和 lifecycle rollback failure 均由对应测试文件中的 specialized fake 表达。
- 在 `docs/maintainability.md` 补充 shared fake 约束和 config/schedule/lifecycle 首选检索边界。
- 验证：`go test ./internal/manager` 通过（173 tests）。
- Review：Standards 与 Spec review 初检指出 shared failure hooks 仍过宽，已全部收敛后复验通过。
