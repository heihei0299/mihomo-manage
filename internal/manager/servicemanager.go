package manager

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

var serviceName = ServiceName

func serviceUnitPath() string {
	path, _ := serviceUnitPathFor(runtime.GOOS)
	return path
}

func serviceUnitPathFor(goos string) (string, error) {
	switch goos {
	case "linux":
		return "/etc/systemd/system/mihomo.service", nil
	case "windows":
		return filepath.Join(managerRoot, "mihomo-service.json"), nil
	default:
		return "", UnsupportedPlatformError{Feature: "service", GOOS: goos}
	}
}

func serviceUnitContent(autoStart bool) []byte {
	content, _ := serviceUnitContentFor(runtime.GOOS, autoStart)
	return content
}

func serviceUnitContentFor(goos string, autoStart bool) ([]byte, error) {
	if _, err := serviceUnitPathFor(goos); err != nil {
		return nil, err
	}
	if goos == "windows" {
		return []byte("{\"name\":\"mihomo\"}\n"), nil
	}
	return []byte(`[Unit]
Description=mihomo (Clash Meta) proxy
After=network.target

[Service]
Type=simple
ExecStart=/opt/mihomo/bin/mihomo -d /opt/mihomo/etc
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`), nil
}

type osStrategy interface {
	isActive(ctx context.Context, name string) (bool, error)
	enable(ctx context.Context, name, serviceFilePath string) error
	disable(ctx context.Context, name string) error
	start(ctx context.Context, name string) error
	stop(ctx context.Context, name string) error
	restart(ctx context.Context, name string) error
	reload(ctx context.Context, name string) error
	isEnabled(ctx context.Context, name string) (bool, error)
	enableAutoStart(ctx context.Context, name, serviceFilePath string) error
	disableAutoStart(ctx context.Context, name string) error
}

type linuxSystemctl struct{ cmd CommandRunner }

func (l linuxSystemctl) isActive(ctx context.Context, name string) (bool, error) {
	out, err := l.cmd.RunCommandIgnoreExit(ctx, "systemctl", "is-active", name)
	if err != nil {
		return false, fmt.Errorf("systemctl is-active: %w", err)
	}
	return strings.TrimSpace(out) == "active", nil
}

func (l linuxSystemctl) enable(ctx context.Context, name, _ string) error {
	if _, err := l.cmd.RunCommand(ctx, "systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	return nil
}

func (l linuxSystemctl) disable(ctx context.Context, name string) error {
	if _, err := l.cmd.RunCommand(ctx, "systemctl", "disable", name); err != nil {
		return fmt.Errorf("systemctl disable %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) start(ctx context.Context, name string) error {
	if _, err := l.cmd.RunCommand(ctx, "systemctl", "start", name); err != nil {
		return fmt.Errorf("systemctl start %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) stop(ctx context.Context, name string) error {
	if _, err := l.cmd.RunCommand(ctx, "systemctl", "stop", name); err != nil {
		return fmt.Errorf("systemctl stop %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) restart(ctx context.Context, name string) error {
	if _, err := l.cmd.RunCommand(ctx, "systemctl", "restart", name); err != nil {
		return fmt.Errorf("systemctl restart %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) reload(ctx context.Context, name string) error {
	if _, err := l.cmd.RunCommand(ctx, "systemctl", "reload", name); err != nil {
		return fmt.Errorf("systemctl reload %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) isEnabled(ctx context.Context, name string) (bool, error) {
	out, err := l.cmd.RunCommandIgnoreExit(ctx, "systemctl", "is-enabled", name)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "enabled", nil
}

func (l linuxSystemctl) enableAutoStart(ctx context.Context, name, _ string) error {
	if _, err := l.cmd.RunCommand(ctx, "systemctl", "enable", name); err != nil {
		return fmt.Errorf("systemctl enable %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) disableAutoStart(ctx context.Context, name string) error {
	if _, err := l.cmd.RunCommand(ctx, "systemctl", "disable", name); err != nil {
		return fmt.Errorf("systemctl disable %s: %w", name, err)
	}
	return nil
}

func strategyFor(cmd CommandRunner, fs FileSystem, os string) osStrategy {
	switch os {
	case "linux":
		return linuxSystemctl{cmd: cmd}
	case "windows":
		return newWindowsServiceStrategy()
	default:
		return nil
	}
}

func (s *OSServiceManager) goos() string {
	if s.osType != "" {
		return s.osType
	}
	return runtime.GOOS
}

func (s *OSServiceManager) strategy() (osStrategy, error) {
	strat := strategyFor(s.cmd, s.fs, s.goos())
	if strat == nil {
		return nil, UnsupportedPlatformError{Feature: "service", GOOS: s.goos()}
	}
	return strat, nil
}

type OSServiceManager struct {
	cmd    CommandRunner
	fs     FileSystem
	osType string
}

func NewOSServiceManager(cmd CommandRunner, fs FileSystem) *OSServiceManager {
	return &OSServiceManager{cmd: cmd, fs: fs}
}

// CheckRegistrationTarget prevents installation from taking ownership of an
// unrelated Windows service with the same name, including a stopped service.
func (s *OSServiceManager) CheckRegistrationTarget(ctx context.Context, name string) error {
	if s.goos() == "windows" {
		return checkWindowsServiceTarget(ctx, name)
	}
	return nil
}

func (s *OSServiceManager) EnableAutoStart(ctx context.Context, name, serviceFilePath string) error {
	return s.withStrategy(ctx, func(ctx context.Context, strat osStrategy) error {
		return strat.enableAutoStart(ctx, name, serviceFilePath)
	})
}

func (s *OSServiceManager) DisableAutoStart(ctx context.Context, name string) error {
	return s.withStrategy(ctx, func(ctx context.Context, strat osStrategy) error { return strat.disableAutoStart(ctx, name) })
}

func (s *OSServiceManager) AutoStartEnabled(ctx context.Context, name string) (bool, error) {
	strat, err := s.strategy()
	if err != nil {
		return false, err
	}
	return strat.isEnabled(ctx, name)
}

func (s *OSServiceManager) withStrategy(ctx context.Context, f func(context.Context, osStrategy) error) error {
	strat, err := s.strategy()
	if err != nil {
		return err
	}
	return f(ctx, strat)
}

func (s *OSServiceManager) IsRunning(ctx context.Context, name string) (bool, error) {
	strat, err := s.strategy()
	if err != nil {
		return false, err
	}
	return strat.isActive(ctx, name)
}

func (s *OSServiceManager) Register(ctx context.Context, name, serviceFilePath string) error {
	return s.withStrategy(ctx, func(ctx context.Context, strat osStrategy) error { return strat.enable(ctx, name, serviceFilePath) })
}

func (s *OSServiceManager) Unregister(ctx context.Context, name string) error {
	return s.withStrategy(ctx, func(ctx context.Context, strat osStrategy) error { return strat.disable(ctx, name) })
}

func (s *OSServiceManager) Start(ctx context.Context, name string) error {
	return s.withStrategy(ctx, func(ctx context.Context, strat osStrategy) error { return strat.start(ctx, name) })
}

func (s *OSServiceManager) Stop(ctx context.Context, name string) error {
	return s.withStrategy(ctx, func(ctx context.Context, strat osStrategy) error { return strat.stop(ctx, name) })
}

func (s *OSServiceManager) Restart(ctx context.Context, name string) error {
	return s.withStrategy(ctx, func(ctx context.Context, strat osStrategy) error { return strat.restart(ctx, name) })
}

func (s *OSServiceManager) Reload(ctx context.Context, name string) error {
	return s.withStrategy(ctx, func(ctx context.Context, strat osStrategy) error { return strat.reload(ctx, name) })
}
