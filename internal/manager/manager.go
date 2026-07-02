package manager

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"runtime"
	"strconv"
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

func New(sys System, svcMgr ServiceManager) Manager {
	return &manager{sys: sys, svcMgr: svcMgr}
}

type manager struct {
	sys    System
	svcMgr ServiceManager

	mu       sync.Mutex
	ticker   *time.Ticker
	stopCh   chan struct{}
}

func (m *manager) Status(ctx context.Context) (*Status, error) {
	binaryPath := "/opt/mihomo/bin/mihomo"
	if !m.sys.FileExists(binaryPath) {
		return &Status{
			Installed:     false,
			InstanceState: Stopped,
		}, nil
	}

	running, err := m.svcMgr.IsRunning("mihomo")
	if err != nil {
		return nil, err
	}

	state := Stopped
	if running {
		state = Running
	}

	version, _ := parseVersion(m.sys, binaryPath)

	return &Status{
		Installed:     true,
		InstanceState: state,
		Version:       version,
	}, nil
}

func RenderConfig(template, subscription, routingRules string) (string, error) {
	result := strings.ReplaceAll(template, "{{subscription}}", subscription)
	result = strings.ReplaceAll(result, "{{routing_rules}}", routingRules)
	return result, nil
}

func timestamp() string {
	return fmt.Sprintf("%d", time.Now().Unix())
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

func parseVersion(sys System, binaryPath string) (string, error) {
	out, err := sys.RunCommand(binaryPath, "-v")
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

const (
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

func (m *manager) resolveVersion(ctx context.Context, version string) string {
	if version != "latest" {
		return version
	}
	tag, err := m.sys.LatestVersion(ctx, "MetaCubeX", "mihomo")
	if err != nil || tag == "" {
		return version
	}
	return tag
}

func (m *manager) Install(ctx context.Context, version string, onProgress ProgressCallback) error {
	version = m.resolveVersion(ctx, version)
	tempPath := fmt.Sprintf("%s.tmp.%s", binaryPath, version)
	gzPath := tempPath + ".gz"

	onProgress(ProgressEvent{Phase: PhaseFetch, Message: fmt.Sprintf("Downloading mihomo %s", version)})
	if err := m.sys.Download(ctx, releaseURL(runtime.GOOS, runtime.GOARCH, version), gzPath); err != nil {
		onProgress(ProgressEvent{Phase: PhaseFetch, Message: "Download failed", Error: err})
		m.sys.Remove(gzPath)
		return fmt.Errorf("download failed: %w", err)
	}
	onProgress(ProgressEvent{Phase: PhaseFetch, Message: "Decompressing"})
	if err := m.decompressGzip(gzPath, tempPath); err != nil {
		m.sys.Remove(gzPath)
		return fmt.Errorf("decompress failed: %w", err)
	}
	m.sys.Remove(gzPath)
	onProgress(ProgressEvent{Phase: PhaseFetch, Message: "Download complete"})

	onProgress(ProgressEvent{Phase: PhaseDeploy, Message: "Deploying binary"})
	if err := m.sys.Rename(tempPath, binaryPath); err != nil {
		m.sys.Remove(tempPath)
		return m.rollbackInstall(ctx, "deploy rename", err)
	}
	onProgress(ProgressEvent{Phase: PhaseDeploy, Message: "Binary deployed"})

	onProgress(ProgressEvent{Phase: PhaseBootstrap, Message: "Creating config directory"})
	if err := m.sys.MkdirAll(configDir, 0755); err != nil {
		return m.rollbackInstall(ctx, "bootstrap mkdir", err)
	}
	if err := m.sys.WriteFile(ConfigTemplatePath, defaultTemplate, 0644); err != nil {
		return m.rollbackInstall(ctx, "bootstrap template", err)
	}
	if err := m.sys.WriteFile(configYAML, defaultConfig, 0644); err != nil {
		return m.rollbackInstall(ctx, "bootstrap config", err)
	}
	svcPath := serviceUnitPath()
	svcContent := serviceUnitContent()
	if err := m.sys.WriteFile(svcPath, svcContent, 0644); err != nil {
		return m.rollbackInstall(ctx, "bootstrap service unit", err)
	}
	onProgress(ProgressEvent{Phase: PhaseBootstrap, Message: "Config files created"})

	onProgress(ProgressEvent{Phase: PhaseRegister, Message: "Registering system service"})
	if err := m.svcMgr.Register(serviceName, svcPath); err != nil {
		return m.rollbackInstall(ctx, "service register", err)
	}
	onProgress(ProgressEvent{Phase: PhaseRegister, Message: "Service registered"})

	onProgress(ProgressEvent{Phase: PhaseStart, Message: "Starting mihomo"})
	if err := m.svcMgr.Start(serviceName); err != nil {
		return m.rollbackInstall(ctx, "service start", err)
	}
	onProgress(ProgressEvent{Phase: PhaseStart, Message: "mihomo is running"})

	return nil
}

func serviceUnitPath() string {
	if runtime.GOOS == "darwin" {
		return "/Library/LaunchAgents/mihomo.plist"
	}
	return "/etc/systemd/system/mihomo.service"
}

func serviceUnitContent() []byte {
	if runtime.GOOS == "darwin" {
		return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>mihomo</string>
  <key>ProgramArguments</key>
  <array>
    <string>/opt/mihomo/bin/mihomo</string>
    <string>-d</string>
    <string>/opt/mihomo/etc</string>
  </array>
  <key>KeepAlive</key>
  <true/>
  <key>RunAtLoad</key>
  <true/>
</dict>
</plist>
`)
	}
	return []byte(`[Unit]
Description=mihomo (Clash Meta) proxy
After=network.target

[Service]
Type=simple
ExecStart=/opt/mihomo/bin/mihomo -d /opt/mihomo/etc
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`)
}

func releaseURL(goos, goarch, version string) string {
	return fmt.Sprintf("https://github.com/MetaCubeX/mihomo/releases/download/%s/mihomo-%s-%s-%s.gz", version, goos, goarch, version)
}

func (m *manager) decompressGzip(src, dest string) error {
	data, err := m.sys.ReadFile(src)
	if err != nil {
		return err
	}
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decompress: %w", err)
	}
	defer gr.Close()
	decompressed, err := io.ReadAll(gr)
	if err != nil {
		return fmt.Errorf("decompress read: %w", err)
	}
	gr.Close()
	return m.sys.WriteFile(dest, decompressed, 0755)
}

func (m *manager) rollbackInstall(ctx context.Context, phase string, err error) error {
	m.svcMgr.Stop(serviceName)
	m.svcMgr.Unregister(serviceName)
	m.sys.Remove(binaryPath)
	m.sys.Remove(configDir)
	return fmt.Errorf("install failed at %s: %w", phase, err)
}

func (m *manager) Uninstall(ctx context.Context, keepBackup bool, onProgress ProgressCallback) error {
	if !m.sys.FileExists(binaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}

	onProgress(ProgressEvent{Phase: PhaseUninstallStop, Message: "Stopping mihomo"})
	if running, _ := m.svcMgr.IsRunning(serviceName); running {
		m.svcMgr.Stop(serviceName)
	}
	onProgress(ProgressEvent{Phase: PhaseUninstallStop, Message: "Stopped"})

	onProgress(ProgressEvent{Phase: PhaseUninstallDeregister, Message: "Removing service"})
	m.svcMgr.Unregister(serviceName)
	onProgress(ProgressEvent{Phase: PhaseUninstallDeregister, Message: "Service removed"})

	onProgress(ProgressEvent{Phase: PhaseUninstallCleanup, Message: "Cleaning up files"})
	if keepBackup {
		backupPath := "/opt/mihomo.bak." + timestamp()
		m.sys.Rename("/opt/mihomo", backupPath)
		onProgress(ProgressEvent{Phase: PhaseUninstallCleanup, Message: "Files backed up to " + backupPath})
	} else {
		m.sys.Remove(binaryPath + ".bak.")
		m.sys.Remove(configDir)
		m.sys.Remove("/opt/mihomo")
		onProgress(ProgressEvent{Phase: PhaseUninstallCleanup, Message: "Files removed"})
	}

	return nil
}

func (m *manager) Start(ctx context.Context) error {
	if !m.sys.FileExists(binaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}
	running, err := m.svcMgr.IsRunning(serviceName)
	if err != nil {
		return err
	}
	if running {
		return fmt.Errorf("mihomo is already running")
	}
	return m.svcMgr.Start(serviceName)
}

func (m *manager) Stop(ctx context.Context) error {
	if !m.sys.FileExists(binaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}
	running, err := m.svcMgr.IsRunning(serviceName)
	if err != nil {
		return err
	}
	if !running {
		return fmt.Errorf("mihomo is not running")
	}
	return m.svcMgr.Stop(serviceName)
}

func (m *manager) Restart(ctx context.Context) error {
	if !m.sys.FileExists(binaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}
	return m.svcMgr.Restart(serviceName)
}

func (m *manager) Reload(ctx context.Context) error {
	if !m.sys.FileExists(binaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}
	running, err := m.svcMgr.IsRunning(serviceName)
	if err != nil {
		return err
	}
	if !running {
		return fmt.Errorf("mihomo is not running")
	}
	return m.svcMgr.Reload(serviceName)
}

func (m *manager) Upgrade(ctx context.Context, version string, onProgress ProgressCallback) error {
	version = m.resolveVersion(ctx, version)
	if !m.sys.FileExists(binaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}

	tempPath := binaryPath + ".tmp." + version
	gzPath := tempPath + ".gz"

	onProgress(ProgressEvent{Phase: PhaseUpgradeFetch, Message: "Downloading " + version})
	if err := m.sys.Download(ctx, releaseURL(runtime.GOOS, runtime.GOARCH, version), gzPath); err != nil {
		m.sys.Remove(gzPath)
		return fmt.Errorf("download failed: %w", err)
	}
	onProgress(ProgressEvent{Phase: PhaseUpgradeFetch, Message: "Decompressing"})
	if err := m.decompressGzip(gzPath, tempPath); err != nil {
		m.sys.Remove(gzPath)
		return fmt.Errorf("decompress failed: %w", err)
	}
	m.sys.Remove(gzPath)
	onProgress(ProgressEvent{Phase: PhaseUpgradeFetch, Message: "Downloaded"})

	onProgress(ProgressEvent{Phase: PhaseUpgradeStop, Message: "Stopping mihomo"})
	if running, _ := m.svcMgr.IsRunning(serviceName); running {
		if err := m.svcMgr.Stop(serviceName); err != nil {
			m.sys.Remove(tempPath)
			return fmt.Errorf("stop failed: %w", err)
		}
	}
	onProgress(ProgressEvent{Phase: PhaseUpgradeStop, Message: "Stopped"})

	onProgress(ProgressEvent{Phase: PhaseUpgradeReplace, Message: "Backing up old binary"})
	backupPath := binaryPath + ".bak." + version
	m.sys.Rename(binaryPath, backupPath)

	onProgress(ProgressEvent{Phase: PhaseUpgradeReplace, Message: "Replacing binary"})
	if err := m.sys.Chmod(tempPath, 0755); err != nil {
		m.restoreBinary(backupPath, tempPath)
		return fmt.Errorf("chmod failed: %w", err)
	}
	if err := m.sys.Rename(tempPath, binaryPath); err != nil {
		m.restoreBinary(backupPath, tempPath)
		return fmt.Errorf("rename failed: %w", err)
	}
	onProgress(ProgressEvent{Phase: PhaseUpgradeReplace, Message: "Binary replaced"})

	onProgress(ProgressEvent{Phase: PhaseUpgradeStart, Message: "Starting mihomo"})
	if err := m.svcMgr.Start(serviceName); err != nil {
		if rbErr := m.restoreBinary(backupPath, ""); rbErr != nil {
			return fmt.Errorf("start failed, rollback also failed: %v (original: %w)", rbErr, err)
		}
		return fmt.Errorf("start failed, rolled back: %w", err)
	}
	onProgress(ProgressEvent{Phase: PhaseUpgradeStart, Message: "Running " + version})

	return nil
}

func (m *manager) restoreBinary(backupPath, tempPath string) error {
	m.sys.Remove(binaryPath)
	m.sys.Rename(backupPath, binaryPath)
	m.sys.Remove(tempPath)
	return m.svcMgr.Start(serviceName)
}

func (m *manager) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	return m.sys.ListVersions(ctx, "MetaCubeX", "mihomo", 5)
}

func (m *manager) SetSubscriptionSource(ctx context.Context, url string) error {
	if looksLikeURL(url) {
		m.sys.WriteFile(subscriptionURLFile, []byte(url), 0644)
	}
	return m.sys.WriteFile(subscriptionDataFile, []byte(url), 0644)
}

func (m *manager) SetRoutingRules(ctx context.Context, rules string) error {
	return m.sys.WriteFile(RoutingRulesPath, []byte(rules), 0644)
}

func (m *manager) PreviewConfig(ctx context.Context) (string, error) {
	tmpl, err := m.sys.ReadFile(ConfigTemplatePath)
	if err != nil {
		return "", err
	}

	var subData []byte
	if m.sys.FileExists(subscriptionDataFile) {
		subData, err = m.sys.ReadFile(subscriptionDataFile)
		if err != nil {
			return "", err
		}
	}

	var rulesData []byte
	if m.sys.FileExists(RoutingRulesPath) {
		rulesData, err = m.sys.ReadFile(RoutingRulesPath)
		if err != nil {
			return "", err
		}
	}

	return RenderConfig(string(tmpl), string(subData), string(rulesData))
}

func (m *manager) UpdateConfig(ctx context.Context) error {
	if m.sys.FileExists(subscriptionURLFile) {
		data, _ := m.sys.ReadFile(subscriptionURLFile)
		url := strings.TrimSpace(string(data))
		if url != "" {
			tmpPath := subscriptionDataFile + ".tmp"
			if err := m.sys.Download(ctx, url, tmpPath); err != nil {
				return fmt.Errorf("fetching subscription: %w", err)
			}
			fetched, err := m.sys.ReadFile(tmpPath)
			if err != nil {
				return err
			}
			m.sys.WriteFile(subscriptionDataFile, fetched, 0644)
			m.sys.Remove(tmpPath)
		}
	}

	preview, err := m.PreviewConfig(ctx)
	if err != nil {
		return err
	}

	if m.sys.FileExists(configYAML) {
		backupPath := configYAML + ".bak." + timestamp()
		existing, err := m.sys.ReadFile(configYAML)
		if err != nil {
			return err
		}
		if err := m.sys.WriteFile(backupPath, existing, 0644); err != nil {
			return err
		}
	}

	if err := m.sys.WriteFile(configYAML, []byte(preview), 0644); err != nil {
		return err
	}

	m.svcMgr.Reload(serviceName)

	return nil
}

func (m *manager) SetSchedule(ctx context.Context, interval time.Duration) error {
	if interval < time.Hour {
		return fmt.Errorf("minimum interval is 1h, got %v", interval)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ticker != nil {
		m.ticker.Stop()
		if m.stopCh != nil {
			close(m.stopCh)
		}
	}

	data := fmt.Sprintf("%d", int64(interval.Seconds()))
	if err := m.sys.WriteFile(scheduleFile, []byte(data), 0644); err != nil {
		return err
	}

	stopCh := make(chan struct{})
	ticker := time.NewTicker(interval)
	m.stopCh = stopCh
	m.ticker = ticker
	go func() {
		for {
			select {
			case <-ticker.C:
				m.UpdateConfig(context.Background())
			case <-stopCh:
				ticker.Stop()
				return
			}
		}
	}()
	return nil
}

func (m *manager) StopSchedule(ctx context.Context) error {
	m.mu.Lock()
	if m.ticker != nil {
		m.ticker.Stop()
	}
	if m.stopCh != nil {
		close(m.stopCh)
		m.stopCh = nil
	}
	m.mu.Unlock()
	m.sys.WriteFile(scheduleFile, []byte("off"), 0644)
	return nil
}

func (m *manager) ScheduleStatus(ctx context.Context) (time.Duration, bool, error) {
	data, err := m.sys.ReadFile(scheduleFile)
	if err != nil {
		return 0, false, nil
	}
	s := strings.TrimSpace(string(data))
	if s == "off" || s == "" {
		return 0, false, nil
	}
	secs, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false, nil
	}
	return time.Duration(secs) * time.Second, true, nil
}
