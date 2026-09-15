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

func NewServiceControl(fs FileSystem, cmd CommandRunner, svcMgr ServiceManager) ServiceControl {
	return &serviceController{fs: fs, cmd: cmd, svcMgr: svcMgr}
}

func (m *serviceController) Status(ctx context.Context) (*Status, error) {
	if !m.fs.FileExists(binaryPath) {
		return &Status{
			Installed:     false,
			InstanceState: Stopped,
		}, nil
	}

	running, err := m.svcMgr.IsRunning(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	state := Stopped
	if running {
		state = Running
	}

	version, versionErr := parseVersion(ctx, m.cmd, binaryPath)
	if versionErr != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	autostart, err := m.svcMgr.AutoStartEnabled(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	return &Status{
		Installed:        true,
		InstanceState:    state,
		Version:          version,
		AutoStartEnabled: autostart,
	}, nil
}

func (m *serviceController) SetAutoStart(ctx context.Context, enabled bool) error {
	if !m.fs.FileExists(binaryPath) {
		return ErrMihomoNotInstalled
	}
	svcPath := serviceUnitPath()
	if enabled {
		return m.svcMgr.EnableAutoStart(ctx, serviceName, svcPath)
	}
	return m.svcMgr.DisableAutoStart(ctx, serviceName)
}

func (m *serviceController) Start(ctx context.Context) error {
	if !m.fs.FileExists(binaryPath) {
		return ErrMihomoNotInstalled
	}
	running, err := m.svcMgr.IsRunning(ctx, serviceName)
	if err != nil {
		return err
	}
	if running {
		return fmt.Errorf("mihomo is already running")
	}
	return m.svcMgr.Start(ctx, serviceName)
}

func (m *serviceController) Stop(ctx context.Context) error {
	if !m.fs.FileExists(binaryPath) {
		return ErrMihomoNotInstalled
	}
	running, err := m.svcMgr.IsRunning(ctx, serviceName)
	if err != nil {
		return err
	}
	if !running {
		return fmt.Errorf("mihomo is not running")
	}
	return m.svcMgr.Stop(ctx, serviceName)
}

func (m *serviceController) Restart(ctx context.Context) error {
	if !m.fs.FileExists(binaryPath) {
		return ErrMihomoNotInstalled
	}
	return m.svcMgr.Restart(ctx, serviceName)
}

func (m *serviceController) Reload(ctx context.Context) error {
	if !m.fs.FileExists(binaryPath) {
		return ErrMihomoNotInstalled
	}
	running, err := m.svcMgr.IsRunning(ctx, serviceName)
	if err != nil {
		return err
	}
	if !running {
		return fmt.Errorf("mihomo is not running")
	}
	return m.svcMgr.Reload(ctx, serviceName)
}
