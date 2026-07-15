package manager

import (
	"context"
	"fmt"
)

type serviceController struct {
	fs     FileSystem
	cmd    CommandRunner
	svcMgr ServiceManager
}

func NewServiceController(fs FileSystem, cmd CommandRunner, svcMgr ServiceManager) ServiceControl {
	return &serviceController{fs: fs, cmd: cmd, svcMgr: svcMgr}
}

func (m *serviceController) Status(ctx context.Context) (*Status, error) {
	if !m.fs.FileExists(binaryPath) {
		return &Status{
			Installed:     false,
			InstanceState: Stopped,
		}, nil
	}

	running, err := m.svcMgr.IsRunning(serviceName)
	if err != nil {
		return nil, err
	}

	state := Stopped
	if running {
		state = Running
	}

	version, _ := parseVersion(m.cmd, binaryPath)
	autostart, _ := m.svcMgr.AutoStartEnabled(serviceName)

	return &Status{
		Installed:        true,
		InstanceState:     state,
		Version:          version,
		AutoStartEnabled: autostart,
	}, nil
}

func (m *serviceController) SetAutoStart(ctx context.Context, enabled bool) error {
	if !m.fs.FileExists(binaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}
	svcPath := serviceUnitPath()
	if enabled {
		return m.svcMgr.EnableAutoStart(serviceName, svcPath)
	}
	return m.svcMgr.DisableAutoStart(serviceName)
}

func (m *serviceController) Start(ctx context.Context) error {
	if !m.fs.FileExists(binaryPath) {
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

func (m *serviceController) Stop(ctx context.Context) error {
	if !m.fs.FileExists(binaryPath) {
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

func (m *serviceController) Restart(ctx context.Context) error {
	if !m.fs.FileExists(binaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}
	return m.svcMgr.Restart(serviceName)
}

func (m *serviceController) Reload(ctx context.Context) error {
	if !m.fs.FileExists(binaryPath) {
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
