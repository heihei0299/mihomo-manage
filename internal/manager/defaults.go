package manager

var defaultOverride = []byte(`# 本地覆写文件（override-file）—— 覆盖 / 补充订阅配置。
#
# 与订阅数据合并的语义：
# - 映射递归深合并；标量直接覆盖订阅值；订阅缺失的字段直接补充
# - 身份数组（proxies、proxy-groups、proxy-providers、rule-providers）
#   按 name 对齐：同名整条替换且位置不变，新名字追加到末尾
# - rules 插入订阅首个 MATCH 之前；订阅没有 MATCH 时追加到末尾
# - !replace 在任意层级整体替换该节点，例如：
#     dns: !replace
#       enable: false
# - !delete 在任意层级删除该键，值必须为 null，例如：
#     dns:
#       fallback: !delete null
# - 数组减项写在数组项上，例如：
#     proxies:
#       - !delete 香港01
#     rules:
#       - !delete DOMAIN-SUFFIX,ads.example.com,DIRECT
# - 删除不存在的键/名字/规则、同一名字既删又增、!replace 子树内再出现
#   !delete 都会报错，错误信息包含路径
#
# 删除本文件后，订阅配置原样生效（纯订阅模式）。

mode: rule
log-level: info

proxy-groups:
  - name: Proxy
    type: select
    include-all: true
    proxies:
      - DIRECT

rules:
  - MATCH,DIRECT
`)

var defaultConfig = []byte(`port: 7890
socks-port: 7891
allow-lan: false
mode: rule
log-level: info
external-controller: 127.0.0.1:9090
`)
