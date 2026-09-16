# 可维护性最小改进执行计划

> 本文件是执行计划；当前维护规则仍以 [`docs/maintainability.md`](maintainability.md) 为准。每项独立、小步，不进行大重构；实际执行测试/构建需另行授权。
>
> 本计划基于 [`docs/maintainability-assessment.md`](maintainability-assessment.md)。逻辑改动遵循 TDD；CI 与纯机械文档改动不强行加测试。所有验证命令仅供获授权执行时使用。

## 总体边界

- M1–M4 是本轮可独立落地的最小范围；M5 是决策门，不自动执行；M6 只做收尾。
- 不拆 manager，不增加 application/usecase 层、DI/CLI 框架或新依赖；不把 acceptance 纳入普通 CI。
- TUI 拆文件仅为条件性延期项，不纳入当前执行。
- 不制定提交数量、时间估算、负责人、KPI、覆盖率阈值或新工具。

## M1：CI 门禁

**目标**：为 PR、`main` push 和发布前构建提供最小 Go 质量门禁。  
**非目标**：不做 reusable workflow、lint 平台或 acceptance 普通门禁；不改变发布产物规则。

**依赖**：无。  
**涉及文件**：新增 `.github/workflows/ci.yml`；修改 `.github/workflows/release.yml`。

**执行步骤**

- [x] 新增 `ci.yml`，触发 `pull_request` 与 `push` 到 `main`。
- [x] 新 CI job 明确使用 `actions/checkout@v4`、`actions/setup-go@v5`（`go-version-file: go.mod`），再执行 `go test ./...`、`go vet ./...`。
- [x] 在 release workflow 增加独立 `check` job，同样使用 `actions/checkout@v4`、`actions/setup-go@v5`（`go-version-file: go.mod`），再执行 `go test ./...`、`go vet ./...`。
- [x] 令现有 matrix build job `needs: check`；保持 release 触发条件和矩阵行为不变。
- [x] 不引入 reusable workflow、第三方 lint 平台或 `acceptance/` job；不改成显式包白名单或新增工具。
- [x] 保持 `acceptance/acceptance_test.go:1`、`acceptance/helpers_test.go:1` 的 `//go:build acceptance` 隔离，使普通 `go test ./...` 不执行 acceptance。

**最小验证命令**：`git diff --check`；获授权后分别运行 `go test ./...`、`go vet ./...`。  
**验收标准**：PR 和 `main` push 均触发 test/vet；release 的 matrix build 只有在独立 `check` 成功后运行；`acceptance/acceptance_test.go:1`、`acceptance/helpers_test.go:1` 保持 `//go:build acceptance`，普通 `go test ./...` 不执行 acceptance；不使用显式包白名单或新工具，YAML 可被 GitHub Actions 解析。  
**风险/回退**：Actions 语法、Go 版本或权限配置错误会阻断门禁；先修正 workflow，必要时回退新增 job/依赖关系，不回退质量命令本身。

**执行记录**：`git diff --check`、`go test ./...`（210 passed in 4 packages）、`go vet ./...` 和 Ruby YAML 解析均通过；`actionlint` 未安装。Standards/Spec Review 均无 blocking finding。

## M2：Start/Restart 不变量归位

**目标**：把 Start/Restart 的配置验证放回 ServiceControl，使所有调用方共享同一不变量，并保持“先验证再调用 control”的可观察顺序。  
**非目标**：不新增 application/usecase 层、decorator 或新接口；不改变控制命令和错误语义。

**依赖**：现有 `ServiceControl` 与配置验证函数。  
**涉及文件**：`internal/manager/service_control.go`、`internal/manager/service_impl.go`、`internal/manager/config*.go`；`main.go`；`internal/cli/handler.go`、`tui.go` 及其对应测试。

**执行步骤**

- [x] 先写失败行为测试：构造 `NewServiceControl` 时显式注入 `func(context.Context) error`，验证 Start/Restart 验证失败时不调用 control；验证成功时先验证、后调用 control。
- [x] 在 `NewServiceControl` 增加该函数参数；在 `serviceController.Start/Restart` 内统一调用验证函数。
- [x] 调整 `main.go`：先构造 config，再构造 control，并注入验证函数。
- [x] 只删除或迁移重复的配置校验实现断言；保留 CLI/TUI 用户可观察输出、错误呈现和 wiring 测试。
- [x] 将保留的测试放在 ServiceControl 接口行为处，明确覆盖验证失败阻止 Start/Restart、验证成功继续、成功顺序和 control 错误透传。
- [x] 按 [`docs/maintainability.md`](maintainability.md) 完成 compact change review record（owner、owner 内/外生产文件、是否读取无关 manager 上下文、shared symbol 是否提升、新跨域依赖、状态/不变量 owner、必要性、现有 role interface 可否复用、是否强化反向依赖）。

**最小验证命令**：`go test ./internal/manager -run 'Test.*(Service|Start|Restart)'`。  
**验收标准**：所有 Start/Restart 路径均经 ServiceControl 验证；验证失败不触发 control，验证成功继续且顺序不变；CLI/TUI 不再重复配置校验实现断言，但保留用户可观察输出、错误呈现和 wiring 测试；无新接口或分层，并有维护文档要求的 compact change review record（见执行步骤）。

**风险/回退**：构造函数调用点遗漏会导致编译失败，验证函数为 nil 可能导致运行时问题；保留显式注入并在构造处传入真实函数，回退时恢复原构造签名和调用方校验，但不保留两套长期逻辑。

**Compact change review record**

Owner: Service
Production files inside owner: `internal/manager/service_impl.go`
Production files outside owner: `main.go`（wiring）、`internal/cli/handler.go`、`tui.go`（caller delegation）
Unrelated manager context required: no
Helper/type promoted to shared: no
New cross-domain dependency: yes; main injects the existing config validation capability as a function seam
State or invariant owner: `ServiceControl.Start/Restart` owns validation-before-control
Why this dependency is necessary: every Start/Restart caller must share the same validation invariant without duplicating config calls
Caller can use an existing role interface: no; using `ConfigManager` would couple ServiceControl to the broader config contract, while the required capability is one function
Reverse dependency introduced or strengthened: no

**执行记录**：局部 manager 测试 36 passed；CLI/root 测试 31 passed；保留 CLI 的 Start/Restart 输出与错误测试以及 TUI 的 control wiring/error propagation 测试。nil validator 在构造处显式拒绝，避免静默绕过配置验证。

## M3：Config 深模块收敛

**目标**：保留 caller-facing 的 `ConfigManager`，让 `configPipeline` 直接实现它，删除无价值的内部转发层。  
**非目标**：不拆包、不新增接口、不改变事务顺序、状态或错误；不重写配置流程。

**依赖**：先确认 `ConfigManager` 的生产调用点和测试替身；可与 M2 独立。  
**涉及文件**：`internal/manager/config_manager.go`、`internal/manager/config.go`、`internal/manager/config_impl.go`、相关 `config_*_test.go` 与构造调用点。

**执行步骤（机械收敛）**

- [x] 搜索全部生产调用点，确认 `ConfigPipeline` 未被生产代码作为独立契约使用。
- [x] 使 `configPipeline` 直接满足 `ConfigManager`；按需将 `Preview/Apply/Adopt/Validate` 重命名为 caller-facing 方法，逐一更新调用点。
- [x] 删除未被生产使用的 `ConfigPipeline` 接口、纯转发 `configManager` 及其无用字段。
- [x] 让 `NewConfigManager` 直接返回 pipeline；内部 options 能收回非导出则一并收回，不扩大 API。
- [x] 保留现有事务步骤、锁/状态更新顺序和错误包装，测试只改因类型/方法名变化而必须改的部分。
- [x] 按 [`docs/maintainability.md`](maintainability.md) 完成 compact change review record（字段要求见 M2 执行步骤）。

**最小验证命令**：`go test ./internal/manager -run 'Test.*Config'`。  
**验收标准**：`ConfigManager` 是唯一保留的 caller-facing 配置契约；生产代码不依赖 `ConfigPipeline` 或纯转发 manager；配置行为、事务顺序、状态和错误完全不变；无新增接口和包；compact change review record 已按维护文档要求完成（见执行步骤）。

**风险/回退**：遗漏的测试或调用点会编译失败，重命名可能误改外部语义；按编译错误和局部测试逐点修正，若行为差异无法解释则回退结构收敛，不改流程以“修测试”。

**Compact change review record**

Owner: Config
Production files inside owner: `internal/manager/config.go`, `internal/manager/config_impl.go`, `internal/manager/adopt.go`
Production files outside owner: none
Unrelated manager context required: no
Helper/type promoted to shared: no
New cross-domain dependency: no
State or invariant owner: `configPipeline` owns configuration rendering, apply transaction, validation, and apply status
Why this dependency is necessary: the pipeline already owns the full config behavior; removing the pure forwarding layer reduces indirection without changing the transaction
Caller can use an existing role interface: not applicable; no new cross-domain call was added
Reverse dependency introduced or strengthened: no

**执行记录**：`go test ./internal/manager -run 'Test.*Config'` 通过（52 passed）；生产代码不再引用 `ConfigPipeline` 或 `configManager`，事务步骤、锁/状态更新和错误包装未改。

## M4：Lifecycle 路径清理

**目标**：集中 lifecycle 的受管路径来源，修正备份清理范围，使卸载删除边界可验证。  
**非目标**：不扩大删除范围、不删除历史时间戳备份、不使用 glob；keep-backup 行为保持不变。

**依赖**：现有 `paths.go`、lifecycle 行为和文件系统 seam。  
**涉及文件**：`internal/manager/paths.go`、`internal/manager/lifecycle_impl.go`、对应 `lifecycle_*_test.go`。

**执行步骤**

- [ ] 先增加聚焦测试：当前 install root 为 `/opt/mihomo`、manager root 为 `/opt/mihomo-manager`；非 keep-backup 只删除这两个根。keep-backup 生成的 `/opt/mihomo.bak.<unix>` 位于两根之外且必须保留；升级回滚备份 `/opt/mihomo-manager/backups/mihomo.bak` 属于 manager root，非 keep-backup 时随 manager root 删除。
- [ ] 在 `paths.go` 只增加或复用最少的 `installRoot`、`managerRoot`、`backupDir` 常量/路径定义。
- [ ] 用这些路径替换 lifecycle 中剩余硬编码 `/opt/...` 路径。
- [ ] 删除无通配能力且误导的 `binaryPath + ".bak."` 条目；实现只删除当前 `installRoot` 与 `managerRoot`，不扫描、匹配或删除 `/opt/mihomo.bak.*`，不得以 glob 或扫描方式补偿它。
- [ ] 运行聚焦测试，精确断言上述路径及删除集合，检查 diff 确认没有扩大数据删除范围。
- [ ] 按 [`docs/maintainability.md`](maintainability.md) 完成 compact change review record（字段要求见 M2 执行步骤）。

**最小验证命令**：`go test ./internal/manager -run 'Test.*(Lifecycle|Uninstall|Backup)'`。  
**验收标准**：仅删除 `/opt/mihomo` 与 `/opt/mihomo-manager`；`/opt/mihomo.bak.<unix>` 始终保留，`/opt/mihomo-manager/backups/mihomo.bak` 随 manager root 删除；删除无效 `binaryPath+".bak."` 条目；不扫描/匹配/删除 `/opt/mihomo.bak.*`，无 glob、递归扩删或额外删除路径；路径定义集中且生命周期状态/错误不变，并有维护文档要求的 compact change review record（见执行步骤）。

**风险/回退**：路径常量错误可能误删或漏删；先由 fake filesystem 测试锁定删除集合，发现范围变化立即回退路径替换，禁止通过更宽删除规则“修复”。

## M5：公共身份与平台支持决策门（不自动执行）

**目标**：由仓库所有者确定 canonical repository 与 Windows 支持边界，再决定后续发布/身份修正。  
**非目标**：未获确认前不改 module、README、仓库设置或 release matrix；M1–M4 不依赖本决策。

**依赖**：仓库所有者明确 D1、D2。  
**涉及文件（依分支）**：`go.mod`、所有 `github.com/.../mihomo-manager` 内部导入、`README.md`、`internal/manager/architecture_test.go`、仓库 remote/设置、`.github/workflows/release.yml`、Windows 平台实现/测试与支持文档。

**执行步骤**

- [ ] D1 确认唯一 canonical repository：A 以当前 origin/README 的 `heihei0299/mihomo-manage` 为准，更新 `go.mod`、内部 import、README/相关仓库引用；或 B 以 anomalyco module 为准，修正 README/仓库设置及相关链接。
- [ ] D1 分支验收：运行 `git remote get-url origin`、`go list -m`，并限定 `rg -n 'heihei0299/mihomo-manage|anomalyco/mihomo-manager' go.mod README.md .github` 与 `rg -n --glob '*.go' 'heihei0299/mihomo-manage|anomalyco/mihomo-manager' .` 检查残留；module、内部 import、README、origin/仓库设置指向同一 canonical 身份。
- [ ] D1 身份分支完成后，获授权运行相关 Go 测试（必要时再运行构建）；未授权不执行。
- [ ] D2 确认 Windows：默认建议在没有服务/调度支持和测试前移除 Windows release matrix；如保留，先定义服务/调度能力、支持边界和测试，并检查 release matrix、README 支持声明、平台实现/测试三者一致。
- [ ] D2-A（移除）影响 `.github/workflows/release.yml` 及 Windows 相关发布文档/承诺；验收为矩阵不发布 Windows 且文档不再承诺未实现能力。
- [ ] D2-B（保留）影响 `.github/workflows/release.yml`、Windows 服务/调度实现、平台测试和支持文档；验收为能力、边界、测试和发布矩阵一致。

**最小验证命令**：获授权且决策完成后运行 `git remote get-url origin`、`go list -m`，以及上述限定 `rg`；身份分支再运行获授权的相关 Go 测试，并检查 Windows release matrix、README 支持声明、平台实现/测试一致性。  
**风险/回退**：身份切换会影响导入方和发布链接，Windows 变更会影响用户预期；未确认不得执行，确认后按对应分支整组回退，不混合另一分支内容。

## M6：文档/工作区收尾

**目标**：在实现完成后同步评估状态，明确历史记录与本地工作区边界。  
**非目标**：不混入功能改动，不借收尾删除未知数据，不改变当前维护规则。

**依赖**：M1–M5 完成；M5 若未决则只保留决策记录。  
**涉及文件**：`docs/maintainability-assessment.md`、历史计划文件、`.gitignore`（仅精确规则时）、`CONTEXT.md`、`.agents/`、`.opencode/`、`.pi/`、`.scratch/`。

**执行步骤**

- [ ] 只在 M1–M5 完成后更新 assessment 状态，保持它是评估快照而非规则来源。
- [ ] 历史计划只有确认无需审计留存后才删除；否则保留并标明状态。
- [ ] 只处理当前逐项确认的未跟踪条目，对 `.agents/.opencode/.pi/.scratch` 等逐项决定 track 或精确 ignore；禁止顺带清理未知工作区内容，不得批量删除或宽泛 ignore。
- [ ] 修正或删除错误项目 context；逐项复核 diff 和工作区状态。

**最小验证命令**：`git diff --check`、`git status --short`。  
**验收标准**：文档状态与实际完成项一致；审计所需历史仍可追溯；未跟踪内容每项有明确结论；无功能文件改动和宽泛忽略规则。  
**风险/回退**：误删会损失审计信息；删除前保留确认依据，发现范围不明即停止并恢复文档/忽略改动。

## 总体验收矩阵

| 项目 | 必须结果 | 依赖决策 |
| --- | --- | --- |
| M1 | PR/main 有 test+vet，release check 阻挡 matrix，acceptance 隔离 | 无 |
| M2 | ServiceControl 统一验证且顺序不变，CLI/TUI 无重复校验 | 无 |
| M3 | pipeline 直接实现 ConfigManager，无内部 pipeline 接口/转发层 | 无 |
| M4 | 路径集中，删除范围精确，时间戳备份保留，无 glob | 无 |
| M5 | D1/D2 有明确分支结果（不决不执行） | 所有者确认 |
| M6 | assessment、历史计划、工作区边界完成收尾 | M1–M5 |

## 停止条件

- M1–M4 完成且矩阵通过即可结束本轮，不必等待 M5 决策。
- 任一测试、构建或静态检查需要实际执行而未获另行授权时停止，不以未运行冒充验收。
- 发现行为、事务顺序、状态或删除范围改变时停止并回退该项。
- TUI 拆文件继续延期，除非未来同时满足维护规则中的拆分信号；本轮不拆 manager、不加 DI/CLI 框架/新依赖、不跑 acceptance 普通 CI。

---

**执行记录原则**：计划只记录执行范围与验收，不替代代码、维护规则或其他当前事实来源。
