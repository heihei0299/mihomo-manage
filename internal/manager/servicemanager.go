package manager

import (
	"fmt"
	"runtime"
	"strings"
)

type osStrategy interface {
	isActive(sys System, name string) (bool, error)
	enable(sys System, name, serviceFilePath string) error
	disable(sys System, name string) error
	start(sys System, name string) error
	stop(sys System, name string) error
	restart(sys System, name string) error
	reload(sys System, name string) error
}

type linuxSystemctl struct{}

func (linuxSystemctl) isActive(sys System, name string) (bool, error) {
	out, err := sys.RunCommand("systemctl", "is-active", name)
	if err != nil {
		return false, fmt.Errorf("systemctl is-active %s: %w", name, err)
	}
	return strings.TrimSpace(out) == "active", nil
}

func (linuxSystemctl) enable(sys System, name, _ string) error {
	if _, err := sys.RunCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %w", err)
	}
	if _, err := sys.RunCommand("systemctl", "enable", name); err != nil {
		return fmt.Errorf("systemctl enable %s: %w", name, err)
	}
	return nil
}

func (linuxSystemctl) disable(sys System, name string) error {
	if _, err := sys.RunCommand("systemctl", "disable", name); err != nil {
		return fmt.Errorf("systemctl disable %s: %w", name, err)
	}
	return nil
}

func (linuxSystemctl) start(sys System, name string) error {
	if _, err := sys.RunCommand("systemctl", "start", name); err != nil {
		return fmt.Errorf("systemctl start %s: %w", name, err)
	}
	return nil
}

func (linuxSystemctl) stop(sys System, name string) error {
	if _, err := sys.RunCommand("systemctl", "stop", name); err != nil {
		return fmt.Errorf("systemctl stop %s: %w", name, err)
	}
	return nil
}

func (linuxSystemctl) restart(sys System, name string) error {
	if _, err := sys.RunCommand("systemctl", "restart", name); err != nil {
		return fmt.Errorf("systemctl restart %s: %w", name, err)
	}
	return nil
}

func (linuxSystemctl) reload(sys System, name string) error {
	if _, err := sys.RunCommand("systemctl", "reload", name); err != nil {
		return fmt.Errorf("systemctl reload %s: %w", name, err)
	}
	return nil
}

type darwinLaunchctl struct{}

func (darwinLaunchctl) isActive(sys System, name string) (bool, error) {
	out, err := sys.RunCommand("launchctl", "list", name)
	if err != nil {
		return false, fmt.Errorf("launchctl list %s: %w", name, err)
	}
	return strings.Contains(out, "PID"), nil
}

func (darwinLaunchctl) enable(sys System, name, serviceFilePath string) error {
	_, err := sys.RunCommand("launchctl", "load", serviceFilePath)
	return err
}

func (darwinLaunchctl) disable(sys System, name string) error {
	_, err := sys.RunCommand("launchctl", "unload", fmt.Sprintf("/Library/LaunchAgents/%s.plist", name))
	return err
}

func (darwinLaunchctl) start(sys System, name string) error {
	_, err := sys.RunCommand("launchctl", "start", name)
	return err
}

func (darwinLaunchctl) stop(sys System, name string) error {
	_, err := sys.RunCommand("launchctl", "stop", name)
	return err
}

func (darwinLaunchctl) restart(sys System, name string) error {
	if _, err := sys.RunCommand("launchctl", "stop", name); err != nil {
		return err
	}
	_, err := sys.RunCommand("launchctl", "start", name)
	return err
}

func (darwinLaunchctl) reload(sys System, name string) error {
	if _, err := sys.RunCommand("launchctl", "stop", name); err != nil {
		return err
	}
	_, err := sys.RunCommand("launchctl", "start", name)
	return err
}

type errUnsupportedOS struct{ os string }

func (e errUnsupportedOS) Error() string { return fmt.Sprintf("unsupported OS: %s", e.os) }

func strategyFor(os string) osStrategy {
	switch os {
	case "linux":
		return linuxSystemctl{}
	case "darwin":
		return darwinLaunchctl{}
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

func (s *OSServiceManager) strategy() osStrategy {
	strat := strategyFor(s.goos())
	if strat == nil {
		return nil
	}
	return strat
}

type OSServiceManager struct {
	sys    System
	osType string
}

func NewOSServiceManager(sys System) *OSServiceManager {
	return &OSServiceManager{sys: sys}
}

func (s *OSServiceManager) IsRunning(name string) (bool, error) {
	strat := s.strategy()
	if strat == nil {
		return false, errUnsupportedOS{s.goos()}
	}
	return strat.isActive(s.sys, name)
}

func (s *OSServiceManager) Register(name, serviceFilePath string) error {
	strat := s.strategy()
	if strat == nil {
		return errUnsupportedOS{s.goos()}
	}
	return strat.enable(s.sys, name, serviceFilePath)
}

func (s *OSServiceManager) Unregister(name string) error {
	strat := s.strategy()
	if strat == nil {
		return errUnsupportedOS{s.goos()}
	}
	return strat.disable(s.sys, name)
}

func (s *OSServiceManager) Start(name string) error {
	strat := s.strategy()
	if strat == nil {
		return errUnsupportedOS{s.goos()}
	}
	return strat.start(s.sys, name)
}

func (s *OSServiceManager) Stop(name string) error {
	strat := s.strategy()
	if strat == nil {
		return errUnsupportedOS{s.goos()}
	}
	return strat.stop(s.sys, name)
}

func (s *OSServiceManager) Restart(name string) error {
	strat := s.strategy()
	if strat == nil {
		return errUnsupportedOS{s.goos()}
	}
	return strat.restart(s.sys, name)
}

func (s *OSServiceManager) Reload(name string) error {
	strat := s.strategy()
	if strat == nil {
		return errUnsupportedOS{s.goos()}
	}
	return strat.reload(s.sys, name)
}
