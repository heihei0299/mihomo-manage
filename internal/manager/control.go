package manager

import (
	"context"
	"fmt"
)

func (m *manager) Status(ctx context.Context) (*Status, error) {
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

func (m *manager) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	return m.sys.ListVersions(ctx, "MetaCubeX", "mihomo", 5)
}
