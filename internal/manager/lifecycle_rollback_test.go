package manager

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type failingWriteFileSystem struct {
	*fakeFileSystem
	err error
}

func (fs *failingWriteFileSystem) WriteFile(path string, data []byte, perm uint32) error {
	return fs.err
}

type failingRemoveFileSystem struct {
	*fakeFileSystem
	err error
}

func (fs *failingRemoveFileSystem) Remove(path string) error {
	return fs.err
}

func TestLifecycleInstallRollbackOnDeployFail(t *testing.T) {
	fs := &failingWriteFileSystem{
		fakeFileSystem: &fakeFileSystem{},
		err:            testError{"disk full"},
	}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{}
	linkStorage(fs.fakeFileSystem, source)
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

func TestInstallRollbackContract(t *testing.T) {
	primary := errors.New("primary failure")
	stopFailure := errors.New("stop rollback failed")
	removeFailure := errors.New("remove rollback failed")

	tests := []struct {
		name        string
		fs          FileSystem
		svc         *mockServiceManager
		wantWrapped error
		wantText    string
	}{
		{
			name: "clean rollback preserves primary",
			fs:   &fakeFileSystem{},
			svc:  &mockServiceManager{},
		},
		{
			name:        "service rollback failure is preserved",
			fs:          &fakeFileSystem{},
			svc:         &mockServiceManager{stopErr: stopFailure},
			wantWrapped: stopFailure,
			wantText:    "rollback failed",
		},
		{
			name: "filesystem rollback failure is preserved",
			fs: &failingRemoveFileSystem{
				fakeFileSystem: &fakeFileSystem{},
				err:            removeFailure,
			},
			svc:         &mockServiceManager{},
			wantWrapped: removeFailure,
			wantText:    "rollback failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &lifecycleManager{fs: tt.fs, svcMgr: tt.svc}
			err := m.rollbackInstall(context.Background(), "test phase", primary)

			if !errors.Is(err, primary) {
				t.Fatalf("rollback error = %v, want primary failure preserved", err)
			}
			if tt.wantWrapped != nil && !errors.Is(err, tt.wantWrapped) {
				t.Fatalf("rollback error = %v, want %v preserved", err, tt.wantWrapped)
			}
			if tt.wantText != "" && !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("rollback error = %v, want %q", err, tt.wantText)
			}
		})
	}
}

func TestInstallDeployFailsRollsBack(t *testing.T) {
	fs := &failingWriteFileSystem{
		fakeFileSystem: &fakeFileSystem{},
		err:            testError{"disk full"},
	}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{}
	linkStorage(fs.fakeFileSystem, source)
	svc := &mockServiceManager{}
	m := NewLifecycleManager(fs, cmd, source, svc)

	var events []ProgressEvent
	err := m.Install(context.Background(), "v1.18.0", true, func(e ProgressEvent) {
		events = append(events, e)
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpgradeStartFailsRollsBack(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{running: true, startErr: testError{"start failed"}}
	m := NewLifecycleManager(fs, cmd, source, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", func(e ProgressEvent) {})
	if err == nil {
		t.Fatal("expected error due to start failure")
	}
}
