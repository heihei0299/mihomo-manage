package manager

import (
	"context"
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
	m := NewLifecycleManager(fs, cmd, source, svc)

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
	m := NewLifecycleManager(fs, cmd, source, svc)

	err := m.Uninstall(context.Background(), false, func(e ProgressEvent) {})
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
	source := &fakeReleaseSource{}
	svc := &mockServiceManager{running: true}
	m := NewLifecycleManager(fs, cmd, source, svc)

	err := m.Uninstall(context.Background(), true, func(e ProgressEvent) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
