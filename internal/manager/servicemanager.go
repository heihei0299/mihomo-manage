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

type linuxSystemctl struct{ cmd CommandRunner }

func (l linuxSystemctl) isActive(name string) (bool, error) {
	out, err := l.cmd.RunCommandIgnoreExit("systemctl", "is-active", name)
	if err != nil {
		return false, fmt.Errorf("systemctl is-active: %w", err)
	}
	return strings.TrimSpace(out) == "active", nil
}

func (l linuxSystemctl) enable(name, _ string) error {
	if _, err := l.cmd.RunCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	if _, err := l.cmd.RunCommand("systemctl", "enable", name); err != nil {
		return fmt.Errorf("systemctl enable %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) disable(name string) error {
	if _, err := l.cmd.RunCommand("systemctl", "disable", name); err != nil {
		return fmt.Errorf("systemctl disable %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) start(name string) error {
	if _, err := l.cmd.RunCommand("systemctl", "start", name); err != nil {
		return fmt.Errorf("systemctl start %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) stop(name string) error {
	if _, err := l.cmd.RunCommand("systemctl", "stop", name); err != nil {
		return fmt.Errorf("systemctl stop %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) restart(name string) error {
	if _, err := l.cmd.RunCommand("systemctl", "restart", name); err != nil {
		return fmt.Errorf("systemctl restart %s: %w", name, err)
	}
	return nil
}

func (l linuxSystemctl) reload(name string) error {
	if _, err := l.cmd.RunCommand("systemctl", "reload", name); err != nil {
		return fmt.Errorf("systemctl reload %s: %w", name, err)
	}
	return nil
}

type darwinLaunchctl struct{ cmd CommandRunner }

func (d darwinLaunchctl) isActive(name string) (bool, error) {
	out, err := d.cmd.RunCommand("launchctl", "list", name)
	if err != nil {
		return false, fmt.Errorf("launchctl list %s: %w", name, err)
	}
	return strings.Contains(out, "PID"), nil
}

func (d darwinLaunchctl) enable(name, serviceFilePath string) error {
	_, err := d.cmd.RunCommand("launchctl", "load", serviceFilePath)
	return err
}

func (d darwinLaunchctl) disable(name string) error {
	_, err := d.cmd.RunCommand("launchctl", "unload", fmt.Sprintf("/Library/LaunchAgents/%s.plist", name))
	return err
}

func (d darwinLaunchctl) start(name string) error {
	_, err := d.cmd.RunCommand("launchctl", "start", name)
	return err
}

func (d darwinLaunchctl) stop(name string) error {
	_, err := d.cmd.RunCommand("launchctl", "stop", name)
	return err
}

func (d darwinLaunchctl) restart(name string) error {
	if _, err := d.cmd.RunCommand("launchctl", "stop", name); err != nil {
		return err
	}
	_, err := d.cmd.RunCommand("launchctl", "start", name)
	return err
}

func (d darwinLaunchctl) reload(name string) error {
	if _, err := d.cmd.RunCommand("launchctl", "stop", name); err != nil {
		return err
	}
	_, err := d.cmd.RunCommand("launchctl", "start", name)
	return err
}

func strategyFor(cmd CommandRunner, os string) osStrategy {
	switch os {
	case "linux":
		return linuxSystemctl{cmd: cmd}
	case "darwin":
		return darwinLaunchctl{cmd: cmd}
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
	strat := strategyFor(s.cmd, s.goos())
	if strat == nil {
		return nil, errUnsupportedOS{s.goos()}
	}
	return strat, nil
}

type errUnsupportedOS struct{ os string }

func (e errUnsupportedOS) Error() string { return fmt.Sprintf("unsupported OS: %s", e.os) }

type OSServiceManager struct {
	cmd    CommandRunner
	osType string
}

func NewOSServiceManager(cmd CommandRunner) *OSServiceManager {
	return &OSServiceManager{cmd: cmd}
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
