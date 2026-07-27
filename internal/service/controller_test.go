package service

import (
	"context"
	"testing"

	"github.com/anomalyco/mihomo-manager/internal/domain"
)

func TestStatusNotInstalled(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{}
	m := NewController(fs, cmd, svc)

	status, err := m.Status(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Installed {
		t.Errorf("expected Installed=false, got true")
	}
	if status.InstanceState != domain.Stopped {
		t.Errorf("expected InstanceState=Stopped, got %v", status.InstanceState)
	}
	if status.Version != "" {
		t.Errorf("expected empty Version, got %q", status.Version)
	}
}

func TestStatusInstalledStopped(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{cmdOutput: "Mihomo Meta v1.18.0 linux amd64"}
	svc := &mockServiceManager{running: false}
	m := NewController(fs, cmd, svc)

	status, err := m.Status(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Installed {
		t.Errorf("expected Installed=true, got false")
	}
	if status.InstanceState != domain.Stopped {
		t.Errorf("expected InstanceState=Stopped, got %v", status.InstanceState)
	}
	if status.Version != "v1.18.0" {
		t.Errorf("expected Version=v1.18.0, got %q", status.Version)
	}
}

func TestStatusInstalledRunning(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{cmdOutput: "Mihomo Meta v1.18.0 linux amd64"}
	svc := &mockServiceManager{running: true}
	m := NewController(fs, cmd, svc)

	status, err := m.Status(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Installed {
		t.Errorf("expected Installed=true, got false")
	}
	if status.InstanceState != domain.Running {
		t.Errorf("expected InstanceState=Running, got %v", status.InstanceState)
	}
}

func TestStatusServiceManagerError(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{err: testError{"service not found"}}
	m := NewController(fs, cmd, svc)

	_, err := m.Status(context.Background())

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestStartNotInstalled(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{}
	m := NewController(fs, cmd, svc)

	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for not installed")
	}
}

func TestStartStopped(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{running: false}
	m := NewController(fs, cmd, svc)

	err := m.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.running {
		t.Error("expected service to be running after Start")
	}
}

func TestStartAlreadyRunning(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{running: true}
	m := NewController(fs, cmd, svc)

	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for already running")
	}
}

func TestStopNotInstalled(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{}
	m := NewController(fs, cmd, svc)

	err := m.Stop(context.Background())
	if err == nil {
		t.Fatal("expected error for not installed")
	}
}

func TestStopRunning(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{running: true}
	m := NewController(fs, cmd, svc)

	err := m.Stop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.running {
		t.Error("expected service to be stopped after Stop")
	}
}

func TestStopAlreadyStopped(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{running: false}
	m := NewController(fs, cmd, svc)

	err := m.Stop(context.Background())
	if err == nil {
		t.Fatal("expected error for already stopped")
	}
}

func TestRestartNotInstalled(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{}
	m := NewController(fs, cmd, svc)

	err := m.Restart(context.Background())
	if err == nil {
		t.Fatal("expected error for not installed")
	}
}

func TestRestartRunning(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{running: true}
	m := NewController(fs, cmd, svc)

	err := m.Restart(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReloadNotInstalled(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{}
	m := NewController(fs, cmd, svc)

	err := m.Reload(context.Background())
	if err == nil {
		t.Fatal("expected error for not installed")
	}
}

func TestReloadRunning(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{running: true}
	m := NewController(fs, cmd, svc)

	err := m.Reload(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReloadStopped(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{running: false}
	m := NewController(fs, cmd, svc)

	err := m.Reload(context.Background())
	if err == nil {
		t.Fatal("expected error for reload when stopped")
	}
}
