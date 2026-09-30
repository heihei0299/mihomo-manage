package manager

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

type rejectedOperationLock struct{ err error }

func (l rejectedOperationLock) Acquire(context.Context) (func(), error) { return nil, l.err }

func TestLifecycleWritersAcquireLockBeforeSideEffects(t *testing.T) {
	for _, name := range []string{"install", "local-install", "upgrade", "uninstall"} {
		t.Run(name, func(t *testing.T) {
			fs := &fakeFileSystem{}
			m := NewLifecycleManager(fs, &fakeCmdRunner{}, &fakeReleaseSource{}, &mockServiceManager{}, noopScheduleManager{}, rejectedOperationLock{ErrInstanceBusy})
			var err error
			switch name {
			case "install":
				err = m.Install(context.Background(), "v2", false, noopProgress)
			case "local-install":
				err = m.InstallFromLocal(context.Background(), "local", false, noopProgress)
			case "upgrade":
				err = m.Upgrade(context.Background(), "v2", noopProgress)
			case "uninstall":
				err = m.Uninstall(context.Background(), false, noopProgress)
			}
			if !errors.Is(err, ErrInstanceBusy) || fs.accessed {
				t.Fatalf("err=%v, filesystem accessed=%v", err, fs.accessed)
			}
		})
	}
}

func TestOverlappingUpgradeCannotOverwriteRecovery(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{binaryPath: true}, written: map[string][]byte{binaryPath: []byte("binary-v1")}}
	svc := &mockServiceManager{running: true}
	aSource := &fakeReleaseSource{downloadData: fakeReleaseArchiveWith("binary-v2")}
	linkStorage(fs, aSource)
	bSource := &fakeReleaseSource{downloadData: fakeReleaseArchiveWith("binary-v3")}
	linkStorage(fs, bSource)
	path := filepath.Join(t.TempDir(), "instance.lock")
	a := NewLifecycleManager(fs, &fakeCmdRunner{}, aSource, svc, noopScheduleManager{}, &fileOperationLock{path: path, busy: ErrInstanceBusy})
	b := NewLifecycleManager(fs, &fakeCmdRunner{}, bSource, svc, noopScheduleManager{}, &fileOperationLock{path: path, busy: ErrInstanceBusy})
	checked := false
	err := a.Upgrade(context.Background(), "v2", func(e ProgressEvent) {
		if e.Phase == PhaseUpgradeReplace && e.Message == "Binary replaced" {
			checked = true
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			if err := b.Upgrade(ctx, "v3", noopProgress); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("overlapping upgrade=%v", err)
			}
		}
	})
	if err != nil || !checked || bSource.downloadCalled || string(fs.written[binaryPath]) != "binary-v2" || string(fs.written[backupDir+"/mihomo.bak"]) != "binary-v1" {
		t.Fatalf("err=%v checked=%v second download=%v binary=%q backup=%q", err, checked, bSource.downloadCalled, fs.written[binaryPath], fs.written[backupDir+"/mihomo.bak"])
	}
	release, err := (&fileOperationLock{path: path}).Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestConfigAndLifecycleShareStableInstanceLock(t *testing.T) {
	if subscriptionUpdateLockFile != instanceOperationLockFile {
		t.Fatal("config and lifecycle lock paths differ")
	}
	if filepath.Dir(instanceOperationLockFile) == stateDir {
		t.Fatal("lock must survive uninstall")
	}
	path := filepath.Join(t.TempDir(), "instance.lock")
	release, err := (&fileOperationLock{path: path}).Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	fs := &fakeFileSystem{}
	cfg := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload, WithConfigUpdateLock(&fileOperationLock{path: path}))
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if err := cfg.UpdateConfig(ctx); !errors.Is(err, context.DeadlineExceeded) || fs.accessed {
		t.Fatalf("config err=%v accessed=%v", err, fs.accessed)
	}
}

func TestLifecycleRequiresOperationLock(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("constructor accepted missing operation lock")
		}
	}()
	NewLifecycleManager(&fakeFileSystem{}, &fakeCmdRunner{}, &fakeReleaseSource{}, &mockServiceManager{}, noopScheduleManager{}, nil)
}
