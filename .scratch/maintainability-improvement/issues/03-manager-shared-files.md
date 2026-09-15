# 03: 收敛 manager 共享职责

**What to build:** 让 manager 共享区域只承担稳定的领域 contract，并把跨域路径与 install/bootstrap 默认内容分别归入明确的职责区域，降低无关修改共同触碰的范围，而不改变 manager package 边界。

**Blocked by:** None (can start immediately)

**Status:** resolved

- [x] 共享区域保留 InstanceState、InstallationPhase、ProgressEvent/Callback、VersionInfo、Status 和 ServiceManager 等共享领域 contract。
- [x] 真正跨域共享的路径集中到明确的 shared path 区域。
- [x] install/bootstrap 生成的默认 override/config 内容集中到明确的 defaults 区域。
- [x] scheduler 专属路径继续由 scheduler 区域负责。
- [x] 共享 contract 区域不再包含默认 YAML 内容。
- [x] 不创建 generic constants.go、utils.go 或 common dumping ground。
- [x] 不为了 timestamp 等少量 helper 建立新的通用抽象。
- [x] manager package 仍保持单包，符号归属和运行时行为不变。

## 实施总结

- 新增 paths.go 与 defaults.go，分别承载跨域路径/权限和 install/bootstrap 默认内容。
- 将 config 专属 legacy template 路径留在 config 区域，将 scheduler 专属 scheduleFile 留在 scheduler 区域。
- 保留 RoutingRulesPath 导出兼容符号，并让测试使用现有 serviceUnitPath。
- 同步维护规则文档的文件归属表。
- 验证：`go test ./internal/manager` 通过。
- Review：Standards 与 Spec review 通过。
