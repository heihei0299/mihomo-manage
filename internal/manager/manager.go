package manager

import (
	"context"
	"fmt"
	"strings"
	"sync"
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
	InstanceState InstanceState
	Installed     bool
	Version       string
}

type ServiceManager interface {
	IsRunning(name string) (bool, error)
	Register(name, serviceFilePath string) error
	Unregister(name string) error
	Start(name string) error
	Stop(name string) error
	Restart(name string) error
	Reload(name string) error
}

type Manager interface {
	Status(ctx context.Context) (*Status, error)
	Install(ctx context.Context, version string, onProgress ProgressCallback) error
	Uninstall(ctx context.Context, keepBackup bool, onProgress ProgressCallback) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	Reload(ctx context.Context) error
	Upgrade(ctx context.Context, version string, onProgress ProgressCallback) error
	ListVersions(ctx context.Context) ([]VersionInfo, error)
	SetSubscriptionSource(ctx context.Context, url string) error
	SetRoutingRules(ctx context.Context, rules string) error
	PreviewConfig(ctx context.Context) (string, error)
	UpdateConfig(ctx context.Context) error
	SetSchedule(ctx context.Context, interval time.Duration) error
	StopSchedule(ctx context.Context) error
	ScheduleStatus(ctx context.Context) (time.Duration, bool, error)
}

func New(fs FileSystem, cmd CommandRunner, gh GitHubReleases, svcMgr ServiceManager) Manager {
	pipe := newConfigPipeline(fs, gh, ConfigPipelineOptions{
		OnReload:  func(ctx context.Context) error { return svcMgr.Reload(serviceName) },
		Validator: &configValidator{},
	})
	return &manager{
		fs:       fs,
		cmd:      cmd,
		gh:       gh,
		svcMgr:   svcMgr,
		pipeline: pipe,
	}
}

type manager struct {
	fs       FileSystem
	cmd      CommandRunner
	gh       GitHubReleases
	svcMgr   ServiceManager
	pipeline *configPipeline

	mu     sync.Mutex
	ticker *time.Ticker
	stopCh chan struct{}
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

	binaryPath           = "/opt/mihomo/bin/mihomo"
	configDir            = "/opt/mihomo/etc"
	ConfigTemplatePath   = "/opt/mihomo/etc/config-template.yaml"
	configYAML           = "/opt/mihomo/etc/config.yaml"
	serviceName          = "mihomo"
	defaultServiceUnitPath = "/etc/systemd/system/mihomo.service"
	stateDir             = "/opt/mihomo-manager/state"
	subscriptionDataFile = "/opt/mihomo-manager/state/subscription-data.txt"
	subscriptionURLFile  = "/opt/mihomo-manager/state/subscription-url.txt"
	RoutingRulesPath     = "/opt/mihomo/etc/rules.txt"
	scheduleFile         = "/opt/mihomo-manager/state/schedule.txt"
)

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
