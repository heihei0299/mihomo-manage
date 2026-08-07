# 01 — mergeYAML 纯函数 + 单元测试

**What to build:** 一个可独立验证的 YAML 深合并纯函数。接收 base（订阅配置）和 overlay（模板配置）两个 YAML 字节串，返回深合并后的 YAML 字节串。合并规则：maps 递归合并、lists overlay 项前置、scalars overlay 覆盖。包含完整的 table-driven 单元测试覆盖所有边角情况。

**Blocked by:** 无 — can start immediately

**Status:** ready-for-agent

- [ ] `go get gopkg.in/yaml.v3` 添加到 go.sum
- [ ] `merge.go` 包含 `mergeYAML(base, overlay []byte) ([]byte, error)` 和 `deepMerge(base, overlay any) any`
- [ ] 列表前置：overlay 列表项在 base 列表项之前
- [ ] map 递归合并：嵌套 map 递归，overlay key 覆盖，base 独有 key 保留
- [ ] 标量覆盖：overlay 值覆盖 base 值
- [ ] 类型不匹配时以 overlay 为准
- [ ] 仅 base 或仅 overlay 存在时返回非空方
- [ ] 空输入返回空结果
- [ ] table-driven 测试覆盖以上所有场景
