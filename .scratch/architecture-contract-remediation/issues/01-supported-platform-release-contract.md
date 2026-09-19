# 01: 建立可信的支持平台与发布契约

**What to build:** 让发布物、运行时平台能力和项目文档表达同一份支持平台契约。用户只能获得真正适用于 Linux amd64、Linux arm64、Darwin amd64 和 Darwin arm64 的管理器；Windows 不再出现在当前发布矩阵中，未支持平台不会回退到其他平台的服务实现。Go module、内部导入、README、release workflow 和架构检查统一使用同一个 canonical repository identity。

**Blocked by:** None (can start immediately).

**Status:** claimed

- [ ] 发布矩阵只包含当前正式支持的 Linux 和 Darwin 目标，并且构建命令显式使用每个目标的 OS 和架构。
- [ ] 每个发布产物的实际目标平台和架构与其名称一致，而不是只改变文件名。
- [ ] Windows 不再生成发布物；未支持平台返回可识别的 unsupported-platform 错误，不使用 systemd 等隐式默认实现。
- [ ] README、Go module、内部导入、release workflow 和架构检查使用同一个 canonical repository identity。
- [ ] 相关 CI 或架构检查能够在不启动真实服务的情况下验证支持平台契约。
- [ ] 不改变 Linux/Darwin 已有的服务、调度和安装行为。

## Comments

### Change review record

Owner: Platform release contract
Production files inside owner: `internal/manager/servicemanager.go`, `internal/manager/service_impl.go`, `internal/manager/native_scheduler.go`, `.github/workflows/release.yml`
Production files outside owner: `internal/manager/lifecycle_impl.go`
Unrelated manager context required: no
Helper/type promoted to shared: no
New cross-domain dependency: no
State or invariant owner: service and scheduler adapters own supported-platform selection; release workflow owns target artifacts
Why this dependency is necessary: lifecycle install must reject unsupported platforms before fetching or writing platform-specific service state
Caller can use an existing role interface: yes; lifecycle already uses `ServiceManager` and `ScheduleManager`
Reverse dependency introduced or strengthened: no
