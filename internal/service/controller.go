package service

import (
	"context"
	"fmt"

	"github.com/anomalyco/mihomo-manager/internal/config"
	"github.com/anomalyco/mihomo-manager/internal/domain"
	"github.com/anomalyco/mihomo-manager/internal/infra"
)

type serviceController struct {
	fs     infra.FileSystem
	cmd    infra.CommandRunner
	svcMgr domain.ServiceManager
}

func NewController(fs infra.FileSystem, cmd infra.CommandRunner, svcMgr domain.ServiceManager) domain.ServiceControl {
	return &serviceController{fs: fs, cmd: cmd, svcMgr: svcMgr}
}

func (m *serviceController) Status(ctx context.Context) (*domain.Status, error) {
	if !m.fs.FileExists(config.BinaryPath) {
		return &domain.Status{
			Installed:     false,
			InstanceState: domain.Stopped,
		}, nil
	}

	running, err := m.svcMgr.IsRunning(config.ServiceName)
	if err != nil {
		return nil, err
	}

	state := domain.Stopped
	if running {
		state = domain.Running
	}

	version, _ := domain.ParseVersion(m.cmd, config.BinaryPath)
	autostart, _ := m.svcMgr.AutoStartEnabled(config.ServiceName)

	return &domain.Status{
		Installed:        true,
		InstanceState:     state,
		Version:          version,
		AutoStartEnabled: autostart,
	}, nil
}

func (m *serviceController) SetAutoStart(ctx context.Context, enabled bool) error {
	if !m.fs.FileExists(config.BinaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}
	svcPath := serviceUnitPath()
	if enabled {
		return m.svcMgr.EnableAutoStart(config.ServiceName, svcPath)
	}
	return m.svcMgr.DisableAutoStart(config.ServiceName)
}

func (m *serviceController) Start(ctx context.Context) error {
	if !m.fs.FileExists(config.BinaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}
	running, err := m.svcMgr.IsRunning(config.ServiceName)
	if err != nil {
		return err
	}
	if running {
		return fmt.Errorf("mihomo is already running")
	}
	return m.svcMgr.Start(config.ServiceName)
}

func (m *serviceController) Stop(ctx context.Context) error {
	if !m.fs.FileExists(config.BinaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}
	running, err := m.svcMgr.IsRunning(config.ServiceName)
	if err != nil {
		return err
	}
	if !running {
		return fmt.Errorf("mihomo is not running")
	}
	return m.svcMgr.Stop(config.ServiceName)
}

func (m *serviceController) Restart(ctx context.Context) error {
	if !m.fs.FileExists(config.BinaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}
	return m.svcMgr.Restart(config.ServiceName)
}

func (m *serviceController) Reload(ctx context.Context) error {
	if !m.fs.FileExists(config.BinaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}
	running, err := m.svcMgr.IsRunning(config.ServiceName)
	if err != nil {
		return err
	}
	if !running {
		return fmt.Errorf("mihomo is not running")
	}
	return m.svcMgr.Reload(config.ServiceName)
}
