package manager

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

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

func TestUninstallNotInstalled(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{}}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{}
	svc := &mockServiceManager{}
	m := NewLifecycleManager(fs, cmd, source, svc, noopScheduleManager{})

	err := m.Uninstall(context.Background(), false, func(e ProgressEvent) {})
	if err == nil {
		t.Fatal("expected error for not installed")
	}
}

func TestUninstallCleanup(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{}
	svc := &mockServiceManager{running: true}
	m := NewLifecycleManager(fs, cmd, source, svc, noopScheduleManager{})

	err := m.Uninstall(context.Background(), false, func(e ProgressEvent) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.stopped {
		t.Error("expected service to be stopped")
	}
}

func TestUninstallRemovesOnlyManagedRoots(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{binaryPath: true},
		written: map[string][]byte{
			"/opt/mihomo-manager/backups/mihomo.bak": []byte("old binary"),
			"/opt/mihomo.bak.123":                    []byte("kept uninstall backup"),
		},
	}
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, &fakeReleaseSource{}, &mockServiceManager{}, noopScheduleManager{})

	if err := m.Uninstall(context.Background(), false, noopProgress); err != nil {
		t.Fatalf("Uninstall failed: %v", err)
	}

	want := []string{"/opt/mihomo", "/opt/mihomo-manager"}
	if len(fs.removed) != len(want) {
		t.Fatalf("removed = %v, want exactly %v", fs.removed, want)
	}
	for i, path := range want {
		if fs.removed[i] != path {
			t.Fatalf("removed = %v, want exactly %v", fs.removed, want)
		}
	}
	if _, ok := fs.written["/opt/mihomo-manager/backups/mihomo.bak"]; ok {
		t.Fatal("manager backup should be removed with manager root")
	}
	if _, ok := fs.written["/opt/mihomo.bak.123"]; !ok {
		t.Fatal("timestamped uninstall backup should be preserved")
	}
}

type uninstallRemoveFailureFileSystem struct {
	*fakeFileSystem
	failPath string
	err      error
}

func (fs *uninstallRemoveFailureFileSystem) RemoveAll(path string) error {
	if path == fs.failPath {
		fs.removed = append(fs.removed, path)
		return fs.err
	}
	return fs.fakeFileSystem.RemoveAll(path)
}

func TestUninstallCleanupErrorPreservesManagedRootOrder(t *testing.T) {
	wantErr := errors.New("manager root unavailable")
	fs := &uninstallRemoveFailureFileSystem{
		fakeFileSystem: &fakeFileSystem{fileExists: map[string]bool{binaryPath: true}},
		failPath:       managerRoot,
		err:            wantErr,
	}
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, &fakeReleaseSource{}, &mockServiceManager{}, noopScheduleManager{})

	err := m.Uninstall(context.Background(), false, noopProgress)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Uninstall error = %v, want %v", err, wantErr)
	}
	wantPaths := []string{installRoot, managerRoot}
	if len(fs.removed) != len(wantPaths) {
		t.Fatalf("remove attempts = %v, want %v", fs.removed, wantPaths)
	}
	for i, path := range wantPaths {
		if fs.removed[i] != path {
			t.Fatalf("remove attempts = %v, want %v", fs.removed, wantPaths)
		}
	}
}

func TestUninstallKeepBackup(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{}
	svc := &mockServiceManager{running: true}
	m := NewLifecycleManager(fs, cmd, source, svc, noopScheduleManager{})

	err := m.Uninstall(context.Background(), true, func(e ProgressEvent) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fs.removed) != 0 {
		t.Fatalf("keep-backup removed = %v, want no removals", fs.removed)
	}
	backupPath, ok := fs.renamed["/opt/mihomo"]
	if !ok {
		t.Fatalf("rename = %v, want install root backup", fs.renamed)
	}
	if !strings.HasPrefix(backupPath, "/opt/mihomo.bak.") || strings.HasPrefix(backupPath, "/opt/mihomo/") {
		t.Fatalf("backup path = %q, want /opt/mihomo.bak.<unix> outside install root", backupPath)
	}
}
