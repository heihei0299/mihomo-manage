package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anomalyco/mihomo-manager/internal/manager"
)

type tuiMockControl struct {
	startCalled   bool
	restartCalled bool
	startErr      error
	restartErr    error
}

func (m *tuiMockControl) Status(ctx context.Context) (*manager.Status, error) {
	return &manager.Status{Installed: true, InstanceState: manager.Running}, nil
}

func (m *tuiMockControl) Start(ctx context.Context) error {
	m.startCalled = true
	return m.startErr
}

func (m *tuiMockControl) Stop(ctx context.Context) error { return nil }

func (m *tuiMockControl) Restart(ctx context.Context) error {
	m.restartCalled = true
	return m.restartErr
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
	updateCalled bool
}

func (m *tuiMockConfig) SetSubscriptionSource(ctx context.Context, url string) error { return nil }

func (m *tuiMockConfig) SetRoutingRules(ctx context.Context, rules string) error { return nil }

func (m *tuiMockConfig) PreviewConfig(ctx context.Context) (string, error) { return "", nil }

func (m *tuiMockConfig) UpdateConfig(ctx context.Context) error {
	m.updateCalled = true
	return nil
}

func (m *tuiMockConfig) ValidateConfig(ctx context.Context) error { return nil }

func (m *tuiMockConfig) LastConfigApply(ctx context.Context) (manager.ConfigApplyStatus, error) {
	return manager.ConfigApplyStatus{State: manager.ConfigUnknown}, nil
}

func (m *tuiMockConfig) AdoptConfig(ctx context.Context, force bool) (manager.AdoptReport, error) {
	return manager.AdoptReport{NoChanges: true}, nil
}

// runActionCmd executes an action command synchronously and returns its error.
func runActionCmd(ctrl manager.ServiceControl, lifecycle manager.LifecycleManager, a action) error {
	cmd := execActionCmd(ctrl, lifecycle, a, nil, "", false)
	msg := cmd()
	done, ok := msg.(actionDoneMsg)
	if !ok {
		return errors.New("expected actionDoneMsg")
	}
	return done.err
}

func TestTUIStatusShowsConfigApplyFailure(t *testing.T) {
	m := model{
		status: &manager.Status{Installed: true, InstanceState: manager.Running},
		configStatus: manager.ConfigApplyStatus{
			State:        manager.ConfigApplyFailed,
			ErrorSummary: "writing staged config failed",
		},
	}

	if got := m.statusView(); !strings.Contains(got, "config: apply-failed") {
		t.Fatalf("status view = %q, want apply-failed status", got)
	}
}

func TestTUIScheduleStatusShowsLegacy(t *testing.T) {
	m := model{
		status:      &manager.Status{Installed: true, InstanceState: manager.Running},
		scheduleErr: fmt.Errorf("schedule lookup: %w", manager.LegacyScheduleError{Interval: time.Hour}),
	}

	if got := m.statusView(); !strings.Contains(got, "schedule: legacy") {
		t.Fatalf("status view = %q, want legacy schedule status", got)
	}
}

func TestTUISubscriptionConfigOffersEditor(t *testing.T) {
	m := model{configTab: configTabSubscription}
	if !strings.Contains(m.configView(), "e) Edit") {
		t.Fatalf("subscription config view should offer an editor: %s", m.configView())
	}
}

func TestEditorCommandPreservesArguments(t *testing.T) {
	cmd, err := editorCommand("code --wait", "/tmp/subscription")
	if err != nil {
		t.Fatalf("editorCommand failed: %v", err)
	}
	if filepath.Base(cmd.Path) != "code" {
		t.Fatalf("editor path = %q, want code", cmd.Path)
	}
	want := []string{"code", "--wait", "/tmp/subscription"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("editor args = %v, want %v", cmd.Args, want)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Fatalf("editor args = %v, want %v", cmd.Args, want)
		}
	}
}

func TestTUIStartPropagatesServiceControlError(t *testing.T) {
	want := errors.New("start failed")
	ctrl := &tuiMockControl{startErr: want}

	if err := runActionCmd(ctrl, &tuiMockLifecycle{}, actStart); !errors.Is(err, want) {
		t.Fatalf("start error = %v, want %v", err, want)
	}
}

func TestTUIStartCallsServiceControl(t *testing.T) {
	ctrl := &tuiMockControl{}

	if err := runActionCmd(ctrl, &tuiMockLifecycle{}, actStart); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctrl.startCalled {
		t.Error("Start should be called")
	}
}

func TestTUIRestartPropagatesServiceControlError(t *testing.T) {
	want := errors.New("restart failed")
	ctrl := &tuiMockControl{restartErr: want}

	if err := runActionCmd(ctrl, &tuiMockLifecycle{}, actRestart); !errors.Is(err, want) {
		t.Fatalf("restart error = %v, want %v", err, want)
	}
}

func TestTUIRestartCallsServiceControl(t *testing.T) {
	ctrl := &tuiMockControl{}

	if err := runActionCmd(ctrl, &tuiMockLifecycle{}, actRestart); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctrl.restartCalled {
		t.Error("Restart should be called")
	}
}
