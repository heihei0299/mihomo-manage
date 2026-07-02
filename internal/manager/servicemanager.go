package manager

import (
	"fmt"
	"runtime"
	"strings"
)

type osStrategy interface {
	isActive(name string) (bool, error)
	enable(name, serviceFilePath string) error
	disable(name string) error
	start(name string) error
	stop(name string) error
	restart(name string) error
	reload(name string) error
}

type linuxSystemctl struct{ sys System }

func (l linuxSystemctl) isActive(name string) (bool, error) {
	out, err := l.sys.RunCommand("systemctl", "is-active", name)
	if err != nil {
		return false, fmt.Errorf("systemctl is-active %s: %w", name, err)
	}
	return strings.TrimSpace(out) == "active", nil
}

func (l linuxSystemctl) enable(name, _ string) error {
	if _, err := l.sys.RunCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	if _, err := l.sys.RunCommand("systemctl", "enable", name); err != nil {
		return fmt.Errorf("systemctl enable %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) disable(name string) error {
	if _, err := l.sys.RunCommand("systemctl", "disable", name); err != nil {
		return fmt.Errorf("systemctl disable %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) start(name string) error {
	if _, err := l.sys.RunCommand("systemctl", "start", name); err != nil {
		return fmt.Errorf("systemctl start %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) stop(name string) error {
	if _, err := l.sys.RunCommand("systemctl", "stop", name); err != nil {
		return fmt.Errorf("systemctl stop %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) restart(name string) error {
	if _, err := l.sys.RunCommand("systemctl", "restart", name); err != nil {
		return fmt.Errorf("systemctl restart %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) reload(name string) error {
	if _, err := l.sys.RunCommand("systemctl", "reload", name); err != nil {
		return fmt.Errorf("systemctl reload %s: %w", name, err)
	}
	return nil
}

type darwinLaunchctl struct{ sys System }

func (d darwinLaunchctl) isActive(name string) (bool, error) {
	out, err := d.sys.RunCommand("launchctl", "list", name)
	if err != nil {
		return false, fmt.Errorf("launchctl list %s: %w", name, err)
	}
	return strings.Contains(out, "PID"), nil
}

func (d darwinLaunchctl) enable(name, serviceFilePath string) error {
	_, err := d.sys.RunCommand("launchctl", "load", serviceFilePath)
	return err
}

func (d darwinLaunchctl) disable(name string) error {
	_, err := d.sys.RunCommand("launchctl", "unload", fmt.Sprintf("/Library/LaunchAgents/%s.plist", name))
	return err
}

func (d darwinLaunchctl) start(name string) error {
	_, err := d.sys.RunCommand("launchctl", "start", name)
	return err
}

func (d darwinLaunchctl) stop(name string) error {
	_, err := d.sys.RunCommand("launchctl", "stop", name)
	return err
}

func (d darwinLaunchctl) restart(name string) error {
	if _, err := d.sys.RunCommand("launchctl", "stop", name); err != nil {
		return err
	}
	_, err := d.sys.RunCommand("launchctl", "start", name)
	return err
}

func (d darwinLaunchctl) reload(name string) error {
	if _, err := d.sys.RunCommand("launchctl", "stop", name); err != nil {
		return err
	}
	_, err := d.sys.RunCommand("launchctl", "start", name)
	return err
}

func strategyFor(sys System, os string) osStrategy {
	switch os {
	case "linux":
		return linuxSystemctl{sys: sys}
	case "darwin":
		return darwinLaunchctl{sys: sys}
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
	strat := strategyFor(s.sys, s.goos())
	if strat == nil {
		return nil, errUnsupportedOS{s.goos()}
	}
	return strat, nil
}

type errUnsupportedOS struct{ os string }

func (e errUnsupportedOS) Error() string { return fmt.Sprintf("unsupported OS: %s", e.os) }

type OSServiceManager struct {
	sys    System
	osType string
}

func NewOSServiceManager(sys System) *OSServiceManager {
	return &OSServiceManager{sys: sys}
}

func (s *OSServiceManager) withStrategy(f func(osStrategy) error) error {
	strat, err := s.strategy()
	if err != nil {
		return err
	}
	return f(strat)
}

func (s *OSServiceManager) IsRunning(name string) (bool, error) {
	strat, err := s.strategy()
	if err != nil {
		return false, err
	}
	return strat.isActive(name)
}

func (s *OSServiceManager) Register(name, serviceFilePath string) error {
	return s.withStrategy(func(strat osStrategy) error { return strat.enable(name, serviceFilePath) })
}

func (s *OSServiceManager) Unregister(name string) error {
	return s.withStrategy(func(strat osStrategy) error { return strat.disable(name) })
}

func (s *OSServiceManager) Start(name string) error {
	return s.withStrategy(func(strat osStrategy) error { return strat.start(name) })
}

func (s *OSServiceManager) Stop(name string) error {
	return s.withStrategy(func(strat osStrategy) error { return strat.stop(name) })
}

func (s *OSServiceManager) Restart(name string) error {
	return s.withStrategy(func(strat osStrategy) error { return strat.restart(name) })
}

func (s *OSServiceManager) Reload(name string) error {
	return s.withStrategy(func(strat osStrategy) error { return strat.reload(name) })
}
