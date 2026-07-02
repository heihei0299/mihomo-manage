package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"mihomo-manager/internal/manager"
)

type mockManager struct {
	statusFn          func() (*manager.Status, error)
	installFn         func(version string, cb manager.ProgressCallback) error
	startFn           func() error
	stopFn            func() error
	previewFn         func() (string, error)
	setSubscriptionFn func(url string) error
	updateConfigFn    func() error
	listVersionsFn    func() ([]manager.VersionInfo, error)
	setScheduleFn     func(interval time.Duration) error
	stopScheduleFn    func() error
	scheduleStatusFn  func() (time.Duration, bool, error)
}

func (m *mockManager) Status(ctx context.Context) (*manager.Status, error) {
	if m.statusFn != nil {
		return m.statusFn()
	}
	return &manager.Status{Installed: true, InstanceState: manager.Running, Version: "v1.0.0"}, nil
}

func (m *mockManager) Install(ctx context.Context, version string, onProgress manager.ProgressCallback) error {
	if m.installFn != nil {
		return m.installFn(version, onProgress)
	}
	return nil
}

func (m *mockManager) Uninstall(ctx context.Context, keepBackup bool, onProgress manager.ProgressCallback) error {
	return nil
}

func (m *mockManager) Start(ctx context.Context) error {
	if m.startFn != nil {
		return m.startFn()
	}
	return nil
}

func (m *mockManager) Stop(ctx context.Context) error {
	if m.stopFn != nil {
		return m.stopFn()
	}
	return nil
}

func (m *mockManager) Restart(ctx context.Context) error { return nil }

func (m *mockManager) Reload(ctx context.Context) error { return nil }

func (m *mockManager) Upgrade(ctx context.Context, version string, onProgress manager.ProgressCallback) error {
	return nil
}

func (m *mockManager) ListVersions(ctx context.Context) ([]manager.VersionInfo, error) {
	if m.listVersionsFn != nil {
		return m.listVersionsFn()
	}
	return []manager.VersionInfo{{Tag: "v1.0.0"}}, nil
}

func (m *mockManager) SetSubscriptionSource(ctx context.Context, url string) error {
	if m.setSubscriptionFn != nil {
		return m.setSubscriptionFn(url)
	}
	return nil
}

func (m *mockManager) SetRoutingRules(ctx context.Context, rules string) error { return nil }

func (m *mockManager) PreviewConfig(ctx context.Context) (string, error) {
	if m.previewFn != nil {
		return m.previewFn()
	}
	return "proxies: test", nil
}

func (m *mockManager) UpdateConfig(ctx context.Context) error {
	if m.updateConfigFn != nil {
		return m.updateConfigFn()
	}
	return nil
}

func (m *mockManager) SetSchedule(ctx context.Context, interval time.Duration) error {
	if m.setScheduleFn != nil {
		return m.setScheduleFn(interval)
	}
	return nil
}

func (m *mockManager) StopSchedule(ctx context.Context) error {
	if m.stopScheduleFn != nil {
		return m.stopScheduleFn()
	}
	return nil
}

func (m *mockManager) ScheduleStatus(ctx context.Context) (time.Duration, bool, error) {
	if m.scheduleStatusFn != nil {
		return m.scheduleStatusFn()
	}
	return 0, false, nil
}

func TestStatusRunning(t *testing.T) {
	var stdout, stderr bytes.Buffer
	h := New(&mockManager{}, &stdout, &stderr)

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
	h := New(&mockManager{
		statusFn: func() (*manager.Status, error) {
			return &manager.Status{Installed: false}, nil
		},
	}, &stdout, &stderr)

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
	h := New(&mockManager{
		statusFn: func() (*manager.Status, error) {
			return nil, errors.New("service not found")
		},
	}, &stdout, &stderr)

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
	h := New(&mockManager{}, &stdout, &stderr)

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
	h := New(&mockManager{
		startFn: func() error { return errors.New("not installed") },
	}, &stdout, &stderr)

	code := h.Start(context.Background())

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "error") {
		t.Errorf("stderr should contain 'error', got %q", stderr.String())
	}
}

func TestPreviewConfig(t *testing.T) {
	var stdout, stderr bytes.Buffer
	h := New(&mockManager{
		previewFn: func() (string, error) {
			return "proxies:\n  - server: example\n", nil
		},
	}, &stdout, &stderr)

	code := h.PreviewConfig(context.Background())

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "example") {
		t.Errorf("stdout should contain config content, got %q", stdout.String())
	}
}
