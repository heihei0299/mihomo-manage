# 02: 按操作拆分生命周期测试

**What to build:** 让生命周期维护者可以分别定位 install、upgrade、uninstall 和 rollback 测试，使部署、二进制替换、清理和恢复契约能够独立 review，同时保留必要的跨流程 integration coverage。

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] install、upgrade、uninstall 和 rollback 测试可以独立定位。
- [x] install 覆盖既有的 download、checksum、local install、deploy、bootstrap、register、auto-start 和 start 场景。
- [x] upgrade 覆盖既有的 checksum-before-stop、download、replace、restart、not-installed 和版本列表场景。
- [x] uninstall 覆盖既有的 scheduler stop、service stop、not-installed、backup choice 和 cleanup 场景。
- [x] rollback 集中覆盖既有的 install/upgrade failure、primary failure、rollback failure 和错误合并场景。
- [x] lifecycle 生产实现不修改。
- [x] 真正跨 install/config/schedule 流程的 integration tests 不因移动而丢失。

## 实施总结

- 将 manager_test 中既有的 install、upgrade、version list、uninstall 场景归入对应 lifecycle 测试组。
- 将 rollback contract 归入 rollback 测试组，将 schedule manager 场景归入 scheduler 测试组。
- 将 lifecycle-wide 测试辅助函数放入跨流程测试组，避免归属于单一操作。
- 验证：`go test ./internal/manager` 通过。
- Review：Standards 与 Spec review 通过；未修改 lifecycle 生产实现。
