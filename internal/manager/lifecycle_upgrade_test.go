package manager

import (
	"context"
	"strings"
	"testing"
)

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

func TestUpgradeDownloadFails(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{downloadErr: testError{"network error"}}
	svc := &mockServiceManager{running: true}
	m := NewLifecycleManager(fs, cmd, source, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", func(e ProgressEvent) {})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpgradeHappyPath(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{running: true}
	m := NewLifecycleManager(fs, cmd, source, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", func(e ProgressEvent) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpgradeNotInstalled(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{}}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{}
	svc := &mockServiceManager{}
	m := NewLifecycleManager(fs, cmd, source, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", func(e ProgressEvent) {})
	if err == nil {
		t.Fatal("expected error for not installed")
	}
}

func TestListVersions(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{
		versions: []VersionInfo{
			{Tag: "v1.19.0"},
			{Tag: "v1.18.0"},
			{Tag: "v1.17.0"},
		},
	}
	svc := &mockServiceManager{}
	m := NewLifecycleManager(fs, cmd, source, svc)

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
