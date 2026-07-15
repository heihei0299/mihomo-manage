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
	ConfigTemplatePath    = "/opt/mihomo/etc/config-template.yaml"
	configYAML            = "/opt/mihomo/etc/config.yaml"
	defaultServiceUnitPath = "/etc/systemd/system/mihomo.service"
	ServiceName           = "mihomo"
	stateDir              = "/opt/mihomo-manager/state"
	subscriptionDataFile  = "/opt/mihomo-manager/state/subscription-data.txt"
	subscriptionURLFile   = "/opt/mihomo-manager/state/subscription-url.txt"
	RoutingRulesPath      = "/opt/mihomo/etc/rules.txt"
	scheduleFile          = "/opt/mihomo-manager/state/schedule.txt"
)

var serviceName = ServiceName

var defaultTemplate = []byte(`port: 7890
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

var defaultConfig = []byte(`port: 7890
socks-port: 7891
allow-lan: false
mode: rule
log-level: info
external-controller: 127.0.0.1:9090
`)
