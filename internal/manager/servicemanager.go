package manager

import (
	"fmt"
	"runtime"
	"strings"
)

func (s *OSServiceManager) goos() string {
	if s.osType != "" {
		return s.osType
	}
	return runtime.GOOS
}

type OSServiceManager struct {
	sys    System
	osType string // "linux" | "darwin", 空则用 s.goos()
}

func NewOSServiceManager(sys System) *OSServiceManager {
	return &OSServiceManager{sys: sys}
}

func (s *OSServiceManager) IsRunning(name string) (bool, error) {
	switch s.goos() {
	case "linux":
		out, err := s.sys.RunCommand("systemctl", "is-active", name)
		if err != nil {
			return false, fmt.Errorf("systemctl is-active %s: %w", name, err)
		}
		return strings.TrimSpace(out) == "active", nil
	case "darwin":
		out, err := s.sys.RunCommand("launchctl", "list", name)
		if err != nil {
			return false, fmt.Errorf("launchctl list %s: %w", name, err)
		}
		return strings.Contains(out, "PID"), nil
	default:
		return false, fmt.Errorf("unsupported OS: %s", s.goos())
	}
}

func (s *OSServiceManager) Register(name, serviceFilePath string) error {
	switch s.goos() {
	case "linux":
		_, err := s.sys.RunCommand("systemctl", "daemon-reload")
		if err != nil {
			return fmt.Errorf("systemctl daemon-reload: %w", err)
		}
		_, err = s.sys.RunCommand("systemctl", "enable", name)
		if err != nil {
			return fmt.Errorf("systemctl enable %s: %w", name, err)
		}
		return nil
	case "darwin":
		_, err := s.sys.RunCommand("launchctl", "load", serviceFilePath)
		return err
	default:
		return fmt.Errorf("unsupported OS: %s", s.goos())
	}
}

func (s *OSServiceManager) Unregister(name string) error {
	switch s.goos() {
	case "linux":
		_, err := s.sys.RunCommand("systemctl", "disable", name)
		if err != nil {
			return fmt.Errorf("systemctl disable %s: %w", name, err)
		}
		return nil
	case "darwin":
		_, err := s.sys.RunCommand("launchctl", "unload", fmt.Sprintf("/Library/LaunchAgents/%s.plist", name))
		return err
	default:
		return fmt.Errorf("unsupported OS: %s", s.goos())
	}
}

func (s *OSServiceManager) Start(name string) error {
	switch s.goos() {
	case "linux":
		_, err := s.sys.RunCommand("systemctl", "start", name)
		if err != nil {
			return fmt.Errorf("systemctl start %s: %w", name, err)
		}
		return nil
	case "darwin":
		_, err := s.sys.RunCommand("launchctl", "start", name)
		return err
	default:
		return fmt.Errorf("unsupported OS: %s", s.goos())
	}
}

func (s *OSServiceManager) Stop(name string) error {
	switch s.goos() {
	case "linux":
		_, err := s.sys.RunCommand("systemctl", "stop", name)
		if err != nil {
			return fmt.Errorf("systemctl stop %s: %w", name, err)
		}
		return nil
	case "darwin":
		_, err := s.sys.RunCommand("launchctl", "stop", name)
		return err
	default:
		return fmt.Errorf("unsupported OS: %s", s.goos())
	}
}

func (s *OSServiceManager) Restart(name string) error {
	switch s.goos() {
	case "linux":
		_, err := s.sys.RunCommand("systemctl", "restart", name)
		if err != nil {
			return fmt.Errorf("systemctl restart %s: %w", name, err)
		}
		return nil
	case "darwin":
		_, err := s.sys.RunCommand("launchctl", "stop", name)
		if err != nil {
			return err
		}
		_, err = s.sys.RunCommand("launchctl", "start", name)
		return err
	default:
		return fmt.Errorf("unsupported OS: %s", s.goos())
	}
}

func (s *OSServiceManager) Reload(name string) error {
	switch s.goos() {
	case "linux":
		_, err := s.sys.RunCommand("systemctl", "reload", name)
		if err != nil {
			return fmt.Errorf("systemctl reload %s: %w", name, err)
		}
		return nil
	case "darwin":
		_, err := s.sys.RunCommand("launchctl", "stop", name)
		if err != nil {
			return err
		}
		_, err = s.sys.RunCommand("launchctl", "start", name)
		return err
	default:
		return fmt.Errorf("unsupported OS: %s", s.goos())
	}
}
