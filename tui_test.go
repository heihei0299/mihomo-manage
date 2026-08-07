package main

import (
	"context"
	"errors"
	"testing"

	"github.com/anomalyco/mihomo-manager/internal/manager"
)

type tuiMockControl struct {
	startCalled   bool
	restartCalled bool
}

func (m *tuiMockControl) Status(ctx context.Context) (*manager.Status, error) {
	return &manager.Status{Installed: true, InstanceState: manager.Running}, nil
}

func (m *tuiMockControl) Start(ctx context.Context) error {
	m.startCalled = true
	return nil
}

func (m *tuiMockControl) Stop(ctx context.Context) error { return nil }

func (m *tuiMockControl) Restart(ctx context.Context) error {
	m.restartCalled = true
	return nil
}

func (m *tuiMockControl) Reload(ctx context.Context) error { return nil }

func (m *tuiMockControl) SetAutoStart(ctx context.Context, enabled bool) error { return nil }

type tuiMockLifecycle struct{}

func (m *tuiMockLifecycle) Install(ctx context.Context, version string, autoStart bool, onProgress manager.ProgressCallback) error {
	return nil
}

func (m *tuiMockLifecycle) InstallFromLocal(ctx context.Context, localPath string, autoStart bool, onProgress manager.ProgressCallback) error {
	return nil
}

func (m *tuiMockLifecycle) Uninstall(ctx context.Context, keepBackup bool, onProgress manager.ProgressCallback) error {
	return nil
}

func (m *tuiMockLifecycle) Upgrade(ctx context.Context, version string, onProgress manager.ProgressCallback) error {
	return nil
}

func (m *tuiMockLifecycle) ListVersions(ctx context.Context) ([]manager.VersionInfo, error) {
	return nil, nil
}

type tuiMockConfig struct {
	validateErr error
}

func (m *tuiMockConfig) SetSubscriptionSource(ctx context.Context, url string) error { return nil }

func (m *tuiMockConfig) SetRoutingRules(ctx context.Context, rules string) error { return nil }

func (m *tuiMockConfig) PreviewConfig(ctx context.Context) (string, error) { return "", nil }

func (m *tuiMockConfig) UpdateConfig(ctx context.Context) error { return nil }

func (m *tuiMockConfig) ValidateConfig(ctx context.Context) error { return m.validateErr }

// runActionCmd executes an action command synchronously and returns its error.
func runActionCmd(ctrl manager.ServiceControl, lifecycle manager.LifecycleManager, cfg manager.ConfigManager, a action) error {
	cmd := execActionCmd(ctrl, lifecycle, cfg, a, nil, "", false)
	msg := cmd()
	done, ok := msg.(actionDoneMsg)
	if !ok {
		return errors.New("expected actionDoneMsg")
	}
	return done.err
}

func TestTUIStartRejectedWhenValidationFails(t *testing.T) {
	ctrl := &tuiMockControl{}
	cfg := &tuiMockConfig{validateErr: errors.New("config invalid")}

	err := runActionCmd(ctrl, &tuiMockLifecycle{}, cfg, actStart)
	if err == nil {
		t.Fatal("expected validation error to reject start")
	}
	if ctrl.startCalled {
		t.Error("Start should not be called when config validation fails")
	}
}

func TestTUIStartProceedsWhenValidationPasses(t *testing.T) {
	ctrl := &tuiMockControl{}
	cfg := &tuiMockConfig{}

	if err := runActionCmd(ctrl, &tuiMockLifecycle{}, cfg, actStart); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctrl.startCalled {
		t.Error("Start should be called when config validation passes")
	}
}

func TestTUIRestartRejectedWhenValidationFails(t *testing.T) {
	ctrl := &tuiMockControl{}
	cfg := &tuiMockConfig{validateErr: errors.New("config invalid")}

	err := runActionCmd(ctrl, &tuiMockLifecycle{}, cfg, actRestart)
	if err == nil {
		t.Fatal("expected validation error to reject restart")
	}
	if ctrl.restartCalled {
		t.Error("Restart should not be called when config validation fails")
	}
}

func TestTUIRestartProceedsWhenValidationPasses(t *testing.T) {
	ctrl := &tuiMockControl{}
	cfg := &tuiMockConfig{}

	if err := runActionCmd(ctrl, &tuiMockLifecycle{}, cfg, actRestart); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctrl.restartCalled {
		t.Error("Restart should be called when config validation passes")
	}
}
