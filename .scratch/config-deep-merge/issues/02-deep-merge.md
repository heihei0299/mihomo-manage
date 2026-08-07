# 02 — 集成深合并到 Preview + 集成测试

**What to build:** 修改 `configPipeline.Preview()`：当模板不含 `{{subscription}}` 占位符且订阅数据存在时，调用 `mergeYAML(subData, tmplData)` 替代当前"直接返回订阅数据"的逻辑。模板含占位符时保持现有字符串替换路径不变。`Apply` 自动继承变更（它调用 `Preview`）。更新 `fakeFileSystem` 集成测试验证完整路径。

**Blocked by:** #01 — mergeYAML 纯函数 + 单元测试

**Status:** ready-for-agent

- [ ] `Preview` 中合并路径改为调用 `mergeYAML(subData, tmplData)`
- [ ] 字符串替换路径（模板含占位符）完全不受影响
- [ ] 无订阅数据时仍返回模板原文
- [ ] 集成测试：订阅 + 模板（无占位符）→ 深合并结果包含双方内容
- [ ] 集成测试：模板含 `{{subscription}}` → 字符串替换（不受影响）
- [ ] 集成测试：无订阅数据 → 模板原文
- [ ] 所有现有 70+ 测试保持绿色
- [ ] `go build . && go vet ./...` 通过
