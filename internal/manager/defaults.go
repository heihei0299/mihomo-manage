package manager

var defaultOverride = []byte(`# 本地覆写文件（override-file）—— 覆盖 / 补充订阅配置。
#
# 与订阅数据合并的语义：
# - 同名标量/映射：本文件的值覆盖订阅的值
# - 订阅缺失的字段：本文件补充
# - 数组字段（proxies、proxy-groups、rules、proxy-providers、rule-providers）
#   默认追加到订阅数组末尾
# - 标 !replace 的数组整体替换订阅数组，例如：
#     proxies: !replace
#       - name: local-only
#         type: ss
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
