package manager

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestLifecycleInstall(t *testing.T) {
	fs := &fakeFileSystem{}
	source := &fakeReleaseSource{}
	svc := &mockServiceManager{}
	m := newLifecycleTestManager(fs, source, svc)

	err := m.Install(context.Background(), "v1.18.0", true, noopProgress)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	assertFileExists(t, fs, binaryPath, "binary should be deployed")
	assertFileExists(t, fs, OverrideFilePath, "template should be created")
	assertFileExists(t, fs, configYAML, "config should be created")
	assertFileExists(t, fs, serviceUnitPath(), "BUG 1: service unit file should be created")
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

func TestInstallDownloadFails(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{downloadErr: testError{"network error"}}
	svc := &mockServiceManager{}
	m := NewLifecycleManager(fs, cmd, source, svc)

	var events []ProgressEvent
	err := m.Install(context.Background(), "v1.18.0", true, func(e ProgressEvent) {
		events = append(events, e)
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(events) == 0 || events[0].Phase != PhaseFetch {
		t.Errorf("expected first event phase to be Fetch, got %v", events)
	}
}

func TestInstallHappyPath(t *testing.T) {
	fs := &fakeFileSystem{}
	source := &fakeReleaseSource{}
	svc := &mockServiceManager{}
	m := newLifecycleTestManager(fs, source, svc)

	var phases []InstallationPhase
	var lastErr error
	err := m.Install(context.Background(), "v1.18.0", true, func(e ProgressEvent) {
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
	if last != PhaseStart {
		t.Errorf("expected last phase to be PhaseStart, got %v", last)
	}
}

func TestInstallCreatesServiceFile(t *testing.T) {
	fs := &fakeFileSystem{}
	source := &fakeReleaseSource{}
	svc := &mockServiceManager{}
	m := newLifecycleTestManager(fs, source, svc)

	m.Install(context.Background(), "v1.18.0", true, func(e ProgressEvent) {})

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
