package manager

import (
	"fmt"
	"strings"
	"time"
)

type InstanceState int

const (
	Stopped   InstanceState = iota
	Running
	Upgrading
)

func (s InstanceState) String() string {
	switch s {
	case Stopped:
		return "stopped"
	case Running:
		return "running"
	case Upgrading:
		return "upgrading"
	default:
		return "unknown"
	}
}

type InstallationPhase int

const (
	PhaseFetch InstallationPhase = iota
	PhaseDeploy
	PhaseBootstrap
	PhaseRegister
	PhaseEnableAutoStart
	PhaseStart
	PhaseUpgradeCheck
	PhaseUpgradeFetch
	PhaseUpgradeStop
	PhaseUpgradeReplace
	PhaseUpgradeStart
	PhaseUninstallStop
	PhaseUninstallDeregister
	PhaseUninstallCleanup
)

func (p InstallationPhase) String() string {
	switch p {
	case PhaseFetch:
		return "fetch"
	case PhaseDeploy:
		return "deploy"
	case PhaseBootstrap:
		return "bootstrap"
	case PhaseRegister:
		return "register"
	case PhaseEnableAutoStart:
		return "enable-auto-start"
	case PhaseStart:
		return "start"
	case PhaseUpgradeCheck:
		return "check"
	case PhaseUpgradeFetch:
		return "fetch"
	case PhaseUpgradeStop:
		return "stop"
	case PhaseUpgradeReplace:
		return "replace"
	case PhaseUpgradeStart:
		return "start"
	case PhaseUninstallStop:
		return "stop"
	case PhaseUninstallDeregister:
		return "deregister"
	case PhaseUninstallCleanup:
		return "cleanup"
	default:
		return "unknown"
	}
}

type ProgressEvent struct {
	Phase   InstallationPhase
	Message string
	Error   error
}

type ProgressCallback func(ProgressEvent)

type VersionInfo struct {
	Tag string
}

type Status struct {
	InstanceState     InstanceState
	Installed         bool
	Version           string
	AutoStartEnabled  bool
}

type ServiceManager interface {
	IsRunning(name string) (bool, error)
	Register(name, serviceFilePath string) error
	Unregister(name string) error
	Start(name string) error
	Stop(name string) error
	Restart(name string) error
	Reload(name string) error
	EnableAutoStart(name, serviceFilePath string) error
	DisableAutoStart(name string) error
	AutoStartEnabled(name string) (bool, error)
}

func NewConfigValidator() ConfigValidator {
	return &configValidator{}
}

func looksLikeURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func looksLikeVersion(s string) bool {
	if len(s) < 2 {
		return false
	}
	if s[0] != 'v' && s[0] != 'V' {
		return false
	}
	return s[1] >= '0' && s[1] <= '9'
}

func parseVersion(cmd CommandRunner, binaryPath string) (string, error) {
	out, err := cmd.RunCommand(binaryPath, "-v")
	if err != nil {
		return "", err
	}
	parts := strings.Fields(out)
	for _, p := range parts {
		if looksLikeVersion(p) {
			return p, nil
		}
	}
	return out, nil
}

func timestamp() string {
	return fmt.Sprintf("%d", time.Now().Unix())
}

const (
	filePermUserRW   = 0644
	filePermUserRWX  = 0755

	binaryPath            = "/opt/mihomo/bin/mihomo"
	configDir             = "/opt/mihomo/etc"
	OverrideFilePath      = "/opt/mihomo/etc/override.yaml"
	legacyTemplatePath    = "/opt/mihomo/etc/config-template.yaml"
	configYAML            = "/opt/mihomo/etc/config.yaml"
	defaultServiceUnitPath = "/etc/systemd/system/mihomo.service"
	ServiceName           = "mihomo"
	stateDir              = "/opt/mihomo-manager/state"
	subscriptionDataFile  = "/opt/mihomo-manager/state/subscription-data.txt"
	subscriptionURLFile    = "/opt/mihomo-manager/state/subscription-url.txt"
	RoutingRulesPath       = "/opt/mihomo/etc/rules.txt"
	scheduleFile           = "/opt/mihomo-manager/state/schedule.txt"
)

var serviceName = ServiceName

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
    proxies:
      - AUTO

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
