# 03: 让远程订阅更新保持源缓存与生成配置一致

**What to build:** 将远程订阅更新实现为一次可恢复的配置 apply 流程：订阅先进入候选 staging，再生成和校验候选配置；只有候选配置成功提交后，新的订阅缓存才会发布。下载失败、校验失败或提交前失败都保留旧配置和旧缓存；reload 失败保留新配置并报告 `pending-reload`。

**Blocked by:** 02: 统一配置写操作的序列化边界

**Status:** claimed

- [ ] 远程订阅下载结果在校验和提交前不会覆盖当前 subscription-data。
- [ ] 候选订阅数据参与候选配置生成和 mihomo 配置校验。
- [ ] 下载失败、生成失败、校验失败、备份失败或提交前失败时，旧配置和旧订阅缓存保持不变。
- [ ] 成功 apply 后，生成配置和订阅缓存对应同一个候选版本。
- [ ] 配置提交成功但 reload 失败时，状态为 `pending-reload`，并保留已提交配置和诊断信息。
- [ ] 配置状态继续区分 validation failure、pre-commit apply failure、pending reload 和 applied。
- [ ] 临时订阅文件、staging 目录和备份清理失败时，保留主错误与恢复错误，并记录可诊断状态。
- [ ] 本地订阅源不发生无关的远程下载或缓存替换。
- [ ] 通过 `ConfigManager` 的外部行为测试覆盖成功、失败、取消、恢复和状态记录，不改变调用方 interface。

## Comments

### Change review record

Owner: Config
Production files inside owner: `internal/manager/config.go`, `internal/manager/config_manager.go`, `internal/manager/adopt.go`
Production files outside owner: none
Unrelated manager context required: no
Helper/type promoted to shared: no
New cross-domain dependency: no
State or invariant owner: config pipeline owns candidate subscription/config commit, recovery, and apply status
Why this dependency is necessary: remote subscription data must remain staged until the generated config is validated and both persisted inputs can be recovered together
Caller can use an existing role interface: yes; callers continue to use `ConfigManager`
Reverse dependency introduced or strengthened: no
