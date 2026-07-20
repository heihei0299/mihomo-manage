package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/anomalyco/mihomo-manager/internal/manager"
)

type mockControl struct {
	statusFn    func() (*manager.Status, error)
	startFn     func() error
	stopFn      func() error
	autoStartFn func(enabled bool) error
}

func (m *mockControl) Status(ctx context.Context) (*manager.Status, error) {
	if m.statusFn != nil {
		return m.statusFn()
	}
	return &manager.Status{Installed: true, InstanceState: manager.Running, Version: "v1.0.0", AutoStartEnabled: true}, nil
}

func (m *mockControl) Start(ctx context.Context) error {
	if m.startFn != nil {
		return m.startFn()
	}
	return nil
}

func (m *mockControl) Stop(ctx context.Context) error {
	if m.stopFn != nil {
		return m.stopFn()
	}
	return nil
}

func (m *mockControl) Restart(ctx context.Context) error { return nil }

func (m *mockControl) Reload(ctx context.Context) error { return nil }

func (m *mockControl) SetAutoStart(ctx context.Context, enabled bool) error {
	if m.autoStartFn != nil {
		return m.autoStartFn(enabled)
	}
	return nil
}

type mockLifecycle struct {
	installFn      func(version string, autoStart bool, cb manager.ProgressCallback) error
	listVersionsFn func() ([]manager.VersionInfo, error)
}

func (m *mockLifecycle) Install(ctx context.Context, version string, autoStart bool, onProgress manager.ProgressCallback) error {
	if m.installFn != nil {
		return m.installFn(version, autoStart, onProgress)
	}
	return nil
}

func (m *mockLifecycle) InstallFromLocal(ctx context.Context, localPath string, autoStart bool, onProgress manager.ProgressCallback) error {
	if m.installFn != nil {
		return m.installFn(localPath, autoStart, onProgress)
	}
	return nil
}

func (m *mockLifecycle) Uninstall(ctx context.Context, keepBackup bool, onProgress manager.ProgressCallback) error {
	return nil
}

func (m *mockLifecycle) Upgrade(ctx context.Context, version string, onProgress manager.ProgressCallback) error {
	return nil
}

func (m *mockLifecycle) ListVersions(ctx context.Context) ([]manager.VersionInfo, error) {
	if m.listVersionsFn != nil {
		return m.listVersionsFn()
	}
	return []manager.VersionInfo{{Tag: "v1.0.0"}}, nil
}

type mockConfig struct {
	previewFn         func() (string, error)
	setSubscriptionFn func(url string) error
	updateConfigFn    func() error
}

func (m *mockConfig) SetSubscriptionSource(ctx context.Context, url string) error {
	if m.setSubscriptionFn != nil {
		return m.setSubscriptionFn(url)
	}
	return nil
}

func (m *mockConfig) SetRoutingRules(ctx context.Context, rules string) error { return nil }

func (m *mockConfig) PreviewConfig(ctx context.Context) (string, error) {
	if m.previewFn != nil {
		return m.previewFn()
	}
	return "proxies: test", nil
}

func (m *mockConfig) UpdateConfig(ctx context.Context) error {
	if m.updateConfigFn != nil {
		return m.updateConfigFn()
	}
	return nil
}

type mockSchedule struct {
	setScheduleFn    func(interval time.Duration) error
	stopScheduleFn   func() error
	scheduleStatusFn func() (time.Duration, bool, error)
}

func (m *mockSchedule) SetSchedule(ctx context.Context, interval time.Duration) error {
	if m.setScheduleFn != nil {
		return m.setScheduleFn(interval)
	}
	return nil
}

func (m *mockSchedule) StopSchedule(ctx context.Context) error {
	if m.stopScheduleFn != nil {
		return m.stopScheduleFn()
	}
	return nil
}

func (m *mockSchedule) ScheduleStatus(ctx context.Context) (time.Duration, bool, error) {
	if m.scheduleStatusFn != nil {
		return m.scheduleStatusFn()
	}
	return 0, false, nil
}

func TestStatusRunning(t *testing.T) {
	var stdout, stderr bytes.Buffer
	h := New(&mockControl{}, &mockLifecycle{}, &mockConfig{}, &mockSchedule{}, &stdout, &stderr)

	code := h.Status(context.Background())

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "running") {
		t.Errorf("stdout should contain 'running', got %q", stdout.String())
	}
}

func TestStatusNotInstalled(t *testing.T) {
	var stdout, stderr bytes.Buffer
	h := New(&mockControl{
		statusFn: func() (*manager.Status, error) {
			return &manager.Status{Installed: false}, nil
		},
	}, &mockLifecycle{}, &mockConfig{}, &mockSchedule{}, &stdout, &stderr)

	code := h.Status(context.Background())

	if code != 2 {
		t.Errorf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stdout.String(), "not installed") {
		t.Errorf("stdout should contain 'not installed', got %q", stdout.String())
	}
}

func TestStatusError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	h := New(&mockControl{
		statusFn: func() (*manager.Status, error) {
			return nil, errors.New("service not found")
		},
	}, &mockLifecycle{}, &mockConfig{}, &mockSchedule{}, &stdout, &stderr)

	code := h.Status(context.Background())

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "error") {
		t.Errorf("stderr should contain 'error', got %q", stderr.String())
	}
}

func TestStart(t *testing.T) {
	var stdout, stderr bytes.Buffer
	h := New(&mockControl{}, &mockLifecycle{}, &mockConfig{}, &mockSchedule{}, &stdout, &stderr)

	code := h.Start(context.Background())

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "started") {
		t.Errorf("stdout should contain 'started', got %q", stdout.String())
	}
}

func TestStartError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	h := New(&mockControl{
		startFn: func() error { return errors.New("not installed") },
	}, &mockLifecycle{}, &mockConfig{}, &mockSchedule{}, &stdout, &stderr)

	code := h.Start(context.Background())

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "error") {
		t.Errorf("stderr should contain 'error', got %q", stderr.String())
	}
}

func TestAutoStartOn(t *testing.T) {
	var stdout, stderr bytes.Buffer
	h := New(&mockControl{}, &mockLifecycle{}, &mockConfig{}, &mockSchedule{}, &stdout, &stderr)

	code := h.AutoStart(context.Background(), true)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "on") {
		t.Errorf("stdout should contain 'on', got %q", stdout.String())
	}
}

func TestAutoStartOff(t *testing.T) {
	var stdout, stderr bytes.Buffer
	h := New(&mockControl{}, &mockLifecycle{}, &mockConfig{}, &mockSchedule{}, &stdout, &stderr)

	code := h.AutoStart(context.Background(), false)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "off") {
		t.Errorf("stdout should contain 'off', got %q", stdout.String())
	}
}

func TestAutoStartError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	h := New(&mockControl{
		autoStartFn: func(enabled bool) error { return errors.New("not installed") },
	}, &mockLifecycle{}, &mockConfig{}, &mockSchedule{}, &stdout, &stderr)

	code := h.AutoStart(context.Background(), true)

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "error") {
		t.Errorf("stderr should contain 'error', got %q", stderr.String())
	}
}

func TestPreviewConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	h := New(&mockControl{}, &mockLifecycle{}, &mockConfig{
		previewFn: func() (string, error) {
			return "proxies:\n  - server: example\n", nil
		},
	}, &mockSchedule{}, &stdout, &stderr)

	code := h.PreviewConfig(context.Background())

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "example") {
		t.Errorf("stdout should contain config content, got %q", stdout.String())
	}
}
