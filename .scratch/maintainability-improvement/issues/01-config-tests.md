# 01: 按行为拆分配置更新测试

**What to build:** 让配置维护者可以按行为快速定位配置更新测试：订阅来源、正常 apply、事务提交、Config Apply 状态和取消/锁竞争分别集中管理，同时保持现有行为契约不变。

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] 配置测试按 source/migration、正常 preview/apply、staging/transaction、apply status、cancellation/lock 五类行为组织。
- [x] 原有测试全部保留，测试名称不无故变化，断言语义、fake 行为和覆盖范围不变；另补充 lock context propagation 的最小回归检查。
- [x] 生产代码不修改。
- [x] Config Applied、Config Pending Reload、Config Validation Failed、Config Apply Failed、cleanup warning 和 Last Config Apply 的状态测试集中且易发现。
- [x] 不为单个测试需求扩大 shared fake。
- [x] download、validation、lock 的取消与 context propagation 场景仍有明确归属。

## 实施总结

- 将原配置更新测试按五类行为拆分，保留原测试与断言语义。
- 复用既有 `recordingValidator`，并将 apply 状态失败断言集中到 status 测试组。
- 验证：`go test ./internal/manager` 通过（173 tests）。
- Review：Standards 与 Spec review 通过；未修改生产代码。
