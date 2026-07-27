package config

// DefaultTemplate is the default config template written during installation.
var DefaultTemplate = []byte(`port: 7890
socks-port: 7891
allow-lan: false
mode: rule
log-level: info
external-controller: 127.0.0.1:9090

proxies:
{{subscription}}

proxy-groups:
  - name: Proxy
    type: select
    proxies:
      - AUTO

rules:
{{routing_rules}}
`)

// DefaultConfig is the default mihomo config used when no template is set.
var DefaultConfig = []byte(`port: 7890
socks-port: 7891
allow-lan: false
mode: rule
log-level: info
external-controller: 127.0.0.1:9090
`)
