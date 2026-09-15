package manager

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func noopProgress(ProgressEvent) {}

func TestLifecycleInstall(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{}
	m := NewLifecycleManager(fs, cmd, source, svc)

	err := m.Install(context.Background(), "v1.18.0", true, noopProgress)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	assertFileExists(t, fs, binaryPath, "binary should be deployed")
	assertFileExists(t, fs, OverrideFilePath, "template should be created")
	assertFileExists(t, fs, configYAML, "config should be created")
	assertFileExists(t, fs, defaultServiceUnitPath, "BUG 1: service unit file should be created")
	if !svc.running {
		t.Error("service should be running after Install")
	}
}

func TestLifecycleInstallRejectsChecksumMismatch(t *testing.T) {
	fs := &fakeFileSystem{}
	source := &fakeReleaseSource{expectedChecksum: strings.Repeat("0", 64)}
	linkStorage(fs, source)
	svc := &mockServiceManager{}
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, source, svc)

	err := m.Install(context.Background(), "v1.18.0", true, noopProgress)
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("Install error = %v, want checksum error", err)
	}
	if _, ok := fs.written[binaryPath]; ok {
		t.Fatal("binary should not be deployed after checksum mismatch")
	}
}

func TestLifecycleInstallFailsClosedWhenChecksumUnavailable(t *testing.T) {
	fs := &fakeFileSystem{}
	source := &fakeReleaseSource{checksumErr: errors.New("checksum metadata unavailable")}
	linkStorage(fs, source)
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, source, &mockServiceManager{})

	if err := m.Install(context.Background(), "v1.18.0", true, noopProgress); err == nil || !strings.Contains(err.Error(), "checksum unavailable") {
		t.Fatalf("Install error = %v, want checksum-unavailable error", err)
	}
	if source.downloadCalled {
		t.Fatal("download should not start without checksum metadata")
	}
}

func TestLifecycleInstallCleansArtifactsAfterDecompressFailure(t *testing.T) {
	fs := &fakeFileSystem{}
	source := &fakeReleaseSource{downloadData: []byte("not gzip")}
	linkStorage(fs, source)
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, source, &mockServiceManager{})

	if err := m.Install(context.Background(), "v1.18.0", true, noopProgress); err == nil || !strings.Contains(err.Error(), "decompress failed") {
		t.Fatalf("Install error = %v, want decompress error", err)
	}
	for _, path := range []string{binaryPath + ".tmp.v1.18.0", binaryPath + ".tmp.v1.18.0.gz"} {
		if _, exists := fs.written[path]; exists {
			t.Fatalf("temporary artifact %q should be removed", path)
		}
	}
}

func TestLifecycleUpgradeRejectsChecksumBeforeStopping(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{binaryPath: true},
		written:    map[string][]byte{binaryPath: []byte("old binary")},
	}
	source := &fakeReleaseSource{expectedChecksum: strings.Repeat("0", 64)}
	linkStorage(fs, source)
	svc := &mockServiceManager{running: true}
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, source, svc)

	if err := m.Upgrade(context.Background(), "v1.18.0", noopProgress); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("Upgrade error = %v, want checksum error", err)
	}
	if svc.stopped {
		t.Fatal("upgrade should verify before stopping the running service")
	}
	if got := string(fs.written[binaryPath]); got != "old binary" {
		t.Fatalf("binary after rejected upgrade = %q", got)
	}
}

func TestLifecycleLocalInstallSkipsRemoteChecksum(t *testing.T) {
	const localPath = "/tmp/mihomo-local"
	fs := &fakeFileSystem{
		fileExists: map[string]bool{localPath: true},
		written:    map[string][]byte{localPath: []byte("local binary")},
	}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, source, &mockServiceManager{})

	if err := m.InstallFromLocal(context.Background(), localPath, false, noopProgress); err != nil {
		t.Fatalf("InstallFromLocal failed: %v", err)
	}
	if source.checksumCalled {
		t.Fatal("local installation should not request a remote checksum")
	}
}

func TestLifecycleLocalInstallHonorsCanceledContext(t *testing.T) {
	const localPath = "/tmp/mihomo-canceled"
	fs := &fakeFileSystem{
		fileExists: map[string]bool{localPath: true},
		written:    map[string][]byte{localPath: []byte("local binary")},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, &fakeReleaseSource{}, &mockServiceManager{})

	if err := m.InstallFromLocal(ctx, localPath, false, noopProgress); err == nil {
		t.Fatal("InstallFromLocal should stop before deployment when context is canceled")
	}
	if _, ok := fs.written[binaryPath]; ok {
		t.Fatal("canceled local install should not deploy the binary")
	}
}

func TestLifecycleInstallThenStatus(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &fakeCmdRunner{cmdOutput: "Mihomo Meta v1.18.0 linux amd64"}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{}
	life := NewLifecycleManager(fs, cmd, source, svc)
	ctrl := NewServiceControl(fs, cmd, svc)

	life.Install(context.Background(), "v1.18.0", true, noopProgress)

	status, err := ctrl.Status(context.Background())
	if err != nil {
		t.Fatalf("Status after install: %v", err)
	}
	if !status.Installed {
		t.Error("Status should show installed after Install")
	}
	if status.InstanceState != Running {
		t.Errorf("Status should be Running, got %v", status.InstanceState)
	}
	if status.Version != "v1.18.0" {
		t.Errorf("Status version should be v1.18.0, got %q", status.Version)
	}
}

func TestLifecycleSubscriptionUpdate(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			OverrideFilePath:    true,
			subscriptionURLFile: true,
			configYAML:          true,
		},
		written: map[string][]byte{
			OverrideFilePath:    []byte("mode: rule\n"),
			subscriptionURLFile: []byte(`https://example.com/sub`),
			configYAML:          []byte(`old config`),
		},
	}
	dl := &fakeDownloader{content: "proxies:\n  - name: node1\n    type: ss"}
	linkStorage(fs, &dl.fakeReleaseSource)
	svc := &mockServiceManager{}
	m := NewConfigManager(fs, dl, &configValidator{}, func(ctx context.Context) error {
		return svc.Reload(context.Background(), serviceName)
	})

	err := m.UpdateConfig(context.Background())
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	assertFileExists(t, fs, configYAML, "config should be updated")
	if !dl.downloadCalled {
		t.Error("BUG 2: remote subscription URL should have been fetched via Download")
	}
	if !svc.reloadCalled {
		t.Error("BUG 3: UpdateConfig should reload service after writing config")
	}
}

func TestLifecycleInstallRollbackOnDeployFail(t *testing.T) {
	fs := &fakeFileSystem{
		writeErr: testError{"disk full"},
	}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{}
	m := NewLifecycleManager(fs, cmd, source, svc)

	err := m.Install(context.Background(), "v1.18.0", true, noopProgress)
	if err == nil {
		t.Fatal("expected Install to fail")
	}

	if svc.running {
		t.Error("service should not be running after failed install")
	}
}

func TestUninstallStopsNativeSchedule(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{binaryPath: true}}
	platform := &fakePlatformScheduler{active: true, interval: time.Hour}
	schedule := NewScheduleManagerWithPlatform(fs, platform, "/opt/mihomo-manager/bin/mihomo-manager")
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, &fakeReleaseSource{}, &mockServiceManager{}, schedule)

	if err := m.Uninstall(context.Background(), false, noopProgress); err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}
	if platform.active {
		t.Fatal("Uninstall should stop the native schedule")
	}
}

func TestScheduleSetAndStop(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{binaryPath: true}}
	m := NewScheduleManagerWithPlatform(fs, &fakePlatformScheduler{}, "/opt/mihomo-manager/bin/mihomo-manager")

	err := m.SetSchedule(context.Background(), time.Hour)
	if err != nil {
		t.Fatalf("SetSchedule failed: %v", err)
	}

	interval, active, err := m.ScheduleStatus(context.Background())
	if err != nil {
		t.Fatalf("ScheduleStatus failed: %v", err)
	}
	if !active {
		t.Error("expected schedule to be active")
	}
	if interval != time.Hour {
		t.Errorf("expected interval 1h, got %v", interval)
	}

	err = m.StopSchedule(context.Background())
	if err != nil {
		t.Fatalf("StopSchedule failed: %v", err)
	}

	_, active, err = m.ScheduleStatus(context.Background())
	if err != nil {
		t.Fatalf("ScheduleStatus after stop failed: %v", err)
	}
	if active {
		t.Error("expected schedule to be inactive after stop")
	}
}

func TestScheduleRejectsShortInterval(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{binaryPath: true}}
	m := NewScheduleManagerWithPlatform(fs, &fakePlatformScheduler{}, "/opt/mihomo-manager/bin/mihomo-manager")

	err := m.SetSchedule(context.Background(), time.Minute)
	if err == nil {
		t.Fatal("expected error for interval < 1h")
	}
}

func TestLifecycleInstallWritesDefaultOverride(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{}
	m := NewLifecycleManager(fs, cmd, source, svc)

	err := m.Install(context.Background(), "v1.18.0", true, noopProgress)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	data, err := fs.ReadFile(OverrideFilePath)
	if err != nil {
		t.Fatalf("override file should be written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "!replace") {
		t.Errorf("default override should document !replace usage, got: %s", content)
	}
	if !strings.Contains(content, "mode: rule") {
		t.Errorf("default override should carry example config, got: %s", content)
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
