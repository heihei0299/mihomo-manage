package service

import (
	"context"
	"strings"
	"testing"

	"github.com/anomalyco/mihomo-manager/internal/config"
	"github.com/anomalyco/mihomo-manager/internal/domain"
)

func noopProgress(domain.ProgressEvent) {}

func TestLifecycleInstall(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{}
	linkStorage(fs, gh)
	svc := &mockServiceManager{}
	m := NewLifecycle(fs, cmd, gh, svc)

	err := m.Install(context.Background(), "v1.18.0", true, noopProgress)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	assertFileExists(t, fs, config.BinaryPath, "binary should be deployed")
	assertFileExists(t, fs, config.ConfigTemplatePath, "template should be created")
	assertFileExists(t, fs, config.ConfigYAML, "config should be created")
	assertFileExists(t, fs, config.DefaultServiceUnitPath, "BUG 1: service unit file should be created")
	if !svc.running {
		t.Error("service should be running after Install")
	}
}

func TestLifecycleInstallThenStatus(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &fakeCmdRunner{cmdOutput: "Mihomo Meta v1.18.0 linux amd64"}
	gh := &fakeGitHubReleases{}
	linkStorage(fs, gh)
	svc := &mockServiceManager{}
	life := NewLifecycle(fs, cmd, gh, svc)
	ctrl := NewController(fs, cmd, svc)

	life.Install(context.Background(), "v1.18.0", true, noopProgress)

	status, err := ctrl.Status(context.Background())
	if err != nil {
		t.Fatalf("Status after install: %v", err)
	}
	if !status.Installed {
		t.Error("Status should show installed after Install")
	}
	if status.InstanceState != domain.Running {
		t.Errorf("Status should be Running, got %v", status.InstanceState)
	}
	if status.Version != "v1.18.0" {
		t.Errorf("Status version should be v1.18.0, got %q", status.Version)
	}
}

func TestLifecycleInstallRollbackOnDeployFail(t *testing.T) {
	fs := &fakeFileSystem{
		writeErr: testError{"disk full"},
	}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{}
	linkStorage(fs, gh)
	svc := &mockServiceManager{}
	m := NewLifecycle(fs, cmd, gh, svc)

	err := m.Install(context.Background(), "v1.18.0", true, noopProgress)
	if err == nil {
		t.Fatal("expected Install to fail")
	}

	if svc.running {
		t.Error("service should not be running after failed install")
	}
}

func TestInstallDownloadFails(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{downloadErr: testError{"network error"}}
	svc := &mockServiceManager{}
	m := NewLifecycle(fs, cmd, gh, svc)

	var events []domain.ProgressEvent
	err := m.Install(context.Background(), "v1.18.0", true, func(e domain.ProgressEvent) {
		events = append(events, e)
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(events) == 0 || events[0].Phase != domain.PhaseFetch {
		t.Errorf("expected first event phase to be Fetch, got %v", events)
	}
}

func TestInstallHappyPath(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{}
	linkStorage(fs, gh)
	svc := &mockServiceManager{}
	m := NewLifecycle(fs, cmd, gh, svc)

	var phases []domain.InstallationPhase
	var lastErr error
	err := m.Install(context.Background(), "v1.18.0", true, func(e domain.ProgressEvent) {
		if e.Error == nil {
			phases = append(phases, e.Phase)
		}
		lastErr = e.Error
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lastErr != nil {
		t.Fatalf("last event had error: %v", lastErr)
	}
	if len(phases) == 0 {
		t.Fatal("expected at least one phase event")
	}
	last := phases[len(phases)-1]
	if last != domain.PhaseStart {
		t.Errorf("expected last phase to be PhaseStart, got %v", last)
	}
}

func TestInstallCreatesServiceFile(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{}
	linkStorage(fs, gh)
	svc := &mockServiceManager{}
	m := NewLifecycle(fs, cmd, gh, svc)

	m.Install(context.Background(), "v1.18.0", true, func(e domain.ProgressEvent) {})

	hasServiceFile := false
	for path := range fs.written {
		if strings.Contains(path, "mihomo.service") || strings.Contains(path, "systemd") {
			hasServiceFile = true
			break
		}
	}
	if !hasServiceFile {
		t.Error("BUG 1: Install did not write a systemd service file — systemctl enable will succeed silently but point at a non-existent unit")
	}
}

func TestInstallDeployFailsRollsBack(t *testing.T) {
	fs := &fakeFileSystem{writeErr: testError{"disk full"}}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{}
	linkStorage(fs, gh)
	svc := &mockServiceManager{}
	m := NewLifecycle(fs, cmd, gh, svc)

	var events []domain.ProgressEvent
	err := m.Install(context.Background(), "v1.18.0", true, func(e domain.ProgressEvent) {
		events = append(events, e)
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpgradeDownloadFails(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{downloadErr: testError{"network error"}}
	svc := &mockServiceManager{running: true}
	m := NewLifecycle(fs, cmd, gh, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", func(e domain.ProgressEvent) {})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpgradeHappyPath(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{}
	linkStorage(fs, gh)
	svc := &mockServiceManager{running: true}
	m := NewLifecycle(fs, cmd, gh, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", func(e domain.ProgressEvent) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpgradeStartFailsRollsBack(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{}
	linkStorage(fs, gh)
	svc := &mockServiceManager{running: true, startErr: testError{"start failed"}}
	m := NewLifecycle(fs, cmd, gh, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", func(e domain.ProgressEvent) {})
	if err == nil {
		t.Fatal("expected error due to start failure")
	}
}

func TestUpgradeNotInstalled(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{}}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{}
	svc := &mockServiceManager{}
	m := NewLifecycle(fs, cmd, gh, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", func(e domain.ProgressEvent) {})
	if err == nil {
		t.Fatal("expected error for not installed")
	}
}

func TestListVersions(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{
		versions: []domain.VersionInfo{
			{Tag: "v1.19.0"},
			{Tag: "v1.18.0"},
			{Tag: "v1.17.0"},
		},
	}
	svc := &mockServiceManager{}
	m := NewLifecycle(fs, cmd, gh, svc)

	versions, err := m.ListVersions(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}
	if versions[0].Tag != "v1.19.0" {
		t.Errorf("expected first version v1.19.0, got %q", versions[0].Tag)
	}
}

func TestUninstallNotInstalled(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{}}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{}
	svc := &mockServiceManager{}
	m := NewLifecycle(fs, cmd, gh, svc)

	err := m.Uninstall(context.Background(), false, func(e domain.ProgressEvent) {})
	if err == nil {
		t.Fatal("expected error for not installed")
	}
}

func TestUninstallCleanup(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{}
	svc := &mockServiceManager{running: true}
	m := NewLifecycle(fs, cmd, gh, svc)

	err := m.Uninstall(context.Background(), false, func(e domain.ProgressEvent) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.stopped {
		t.Error("expected service to be stopped")
	}
}

func TestUninstallKeepBackup(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{}
	svc := &mockServiceManager{running: true}
	m := NewLifecycle(fs, cmd, gh, svc)

	err := m.Uninstall(context.Background(), true, func(e domain.ProgressEvent) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertFileExists(t *testing.T, fs *fakeFileSystem, path string, msg string) {
	t.Helper()
	if fs.written != nil {
		if _, ok := fs.written[path]; ok {
			return
		}
	}
	for _, newPath := range fs.renamed {
		if newPath == path {
			return
		}
	}
	t.Errorf("%s: expected %q to exist (written or renamed)", msg, path)
}
