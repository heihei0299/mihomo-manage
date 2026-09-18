package manager

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type upgradeTestFileSystem struct {
	*fakeFileSystem
	failOnRename int
	renameErr    error
	mkdirErr     error
	cancel       context.CancelFunc
	renameCalls  int
}

func (fs *upgradeTestFileSystem) Rename(oldPath, newPath string) error {
	fs.renameCalls++
	if fs.failOnRename == fs.renameCalls {
		return fs.renameErr
	}
	if err := fs.fakeFileSystem.Rename(oldPath, newPath); err != nil {
		return err
	}
	if fs.cancel != nil && fs.renameCalls == 2 {
		fs.cancel()
		fs.cancel = nil
	}
	return nil
}

func (fs *upgradeTestFileSystem) MkdirAll(path string, perm uint32) error {
	if fs.mkdirErr != nil {
		return fs.mkdirErr
	}
	return fs.fakeFileSystem.MkdirAll(path, perm)
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

func TestUpgradeDownloadFails(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{binaryPath: true},
		written:    map[string][]byte{binaryPath: []byte("old binary")},
	}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{downloadErr: testError{"network error"}}
	linkStorage(fs, source)
	svc := &mockServiceManager{running: true}
	m := NewLifecycleManager(fs, cmd, source, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", func(e ProgressEvent) {})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if svc.stopped {
		t.Fatal("download failure must not stop the running service")
	}
	if got := string(fs.written[binaryPath]); got != "old binary" {
		t.Fatalf("binary after rejected upgrade = %q", got)
	}
}

func TestUpgradeLatestLookupFailsBeforeStopping(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{binaryPath: true},
		written:    map[string][]byte{binaryPath: []byte("old binary")},
	}
	source := &fakeReleaseSource{latestErr: testError{"release lookup failed"}}
	linkStorage(fs, source)
	svc := &mockServiceManager{running: true}
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, source, svc)

	err := m.Upgrade(context.Background(), "latest", noopProgress)
	if err == nil || !strings.Contains(err.Error(), "latest") {
		t.Fatalf("Upgrade error = %v, want latest lookup error", err)
	}
	if source.latestCalls != 1 {
		t.Fatalf("latest lookup calls = %d, want 1", source.latestCalls)
	}
	if svc.stopped {
		t.Fatal("latest lookup failure must not stop the running service")
	}
	if got := string(fs.written[binaryPath]); got != "old binary" {
		t.Fatalf("binary after rejected upgrade = %q", got)
	}
}

func TestUpgradeReportsCheckAndFetchPhases(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{binaryPath: true}}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{running: false}
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, source, svc)
	var phases []InstallationPhase

	if err := m.Upgrade(context.Background(), "v1.19.0", func(e ProgressEvent) {
		phases = append(phases, e.Phase)
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !containsPhase(phases, PhaseUpgradeCheck) {
		t.Fatalf("phases = %v, want check phase", phases)
	}
	if !containsPhase(phases, PhaseUpgradeFetch) {
		t.Fatalf("phases = %v, want fetch phase", phases)
	}
}

func containsPhase(phases []InstallationPhase, want InstallationPhase) bool {
	for _, phase := range phases {
		if phase == want {
			return true
		}
	}
	return false
}

func TestUpgradeHappyPath(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{binaryPath: true},
		written:    map[string][]byte{binaryPath: []byte("old binary")},
	}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{running: true}
	m := newLifecycleTestManager(fs, source, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", func(e ProgressEvent) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.running {
		t.Fatal("running instance should be running after a successful upgrade")
	}
	if got := string(fs.written[backupDir+"/mihomo.bak"]); got != "old binary" {
		t.Fatalf("backup binary = %q, want old binary", got)
	}
}

func TestUpgradeStoppedInstanceRemainsStopped(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{binaryPath: true},
		written:    map[string][]byte{binaryPath: []byte("old binary")},
	}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{running: false}
	m := newLifecycleTestManager(fs, source, svc)

	if err := m.Upgrade(context.Background(), "v1.19.0", noopProgress); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.running {
		t.Fatal("stopped instance should remain stopped after a successful upgrade")
	}
}

func TestUpgradeReplacesRecentBinaryBackup(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{binaryPath: true},
		written:    map[string][]byte{binaryPath: []byte("old binary")},
	}
	source := &fakeReleaseSource{downloadData: fakeReleaseArchiveWith("new binary one")}
	linkStorage(fs, source)
	svc := &mockServiceManager{}
	m := newLifecycleTestManager(fs, source, svc)

	if err := m.Upgrade(context.Background(), "v1.19.0", noopProgress); err != nil {
		t.Fatalf("first upgrade: %v", err)
	}
	source.downloadData = fakeReleaseArchiveWith("new binary two")
	if err := m.Upgrade(context.Background(), "v1.20.0", noopProgress); err != nil {
		t.Fatalf("second upgrade: %v", err)
	}
	if got := string(fs.written[backupDir+"/mihomo.bak"]); got != "new binary one" {
		t.Fatalf("recent backup = %q, want previous binary", got)
	}
	if got := string(fs.written[binaryPath]); got != "new binary two" {
		t.Fatalf("current binary = %q, want latest binary", got)
	}
}

func TestUpgradeRequiresServiceToBeRunningAfterStart(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{binaryPath: true},
		written:    map[string][]byte{binaryPath: []byte("old binary")},
	}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{running: true, startDoesNotRun: true}
	m := newLifecycleTestManager(fs, source, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", noopProgress)
	if err == nil || !strings.Contains(err.Error(), "running") {
		t.Fatalf("Upgrade error = %v, want post-start running error", err)
	}
}

func TestUpgradeRequiresStoppedServiceToRemainStopped(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{binaryPath: true},
		written:    map[string][]byte{binaryPath: []byte("old binary")},
	}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{runningStates: []bool{false, true}}
	m := newLifecycleTestManager(fs, source, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", noopProgress)
	if err == nil || !strings.Contains(err.Error(), "stopped") {
		t.Fatalf("Upgrade error = %v, want post-upgrade stopped-state error", err)
	}
}

func TestUpgradeRollbackStopsUnexpectedRunningService(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{binaryPath: true},
		written:    map[string][]byte{binaryPath: []byte("old binary")},
	}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{
		running:        true,
		reportedStates: []bool{true, false, true},
	}
	m := newLifecycleTestManager(fs, source, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", noopProgress)
	if err == nil || !strings.Contains(err.Error(), "running") {
		t.Fatalf("Upgrade error = %v, want post-start confirmation error", err)
	}
	if got := string(fs.written[binaryPath]); got != "old binary" {
		t.Fatalf("binary after rollback = %q, want old binary", got)
	}
	if !svc.running {
		t.Fatal("rollback should restore the running instance")
	}
	if svc.stopCalls < 2 {
		t.Fatalf("stop calls = %d, want rollback to stop the replacement service", svc.stopCalls)
	}
}

func TestUpgradeStopFailureLeavesInstanceUntouched(t *testing.T) {
	primary := errors.New("stop failed")
	fs := &fakeFileSystem{
		fileExists: map[string]bool{binaryPath: true},
		written:    map[string][]byte{binaryPath: []byte("old binary")},
	}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{running: true, stopErr: primary}
	m := newLifecycleTestManager(fs, source, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", noopProgress)
	if !errors.Is(err, primary) {
		t.Fatalf("Upgrade error = %v, want stop error", err)
	}
	if !svc.running || svc.stopCalls != 1 {
		t.Fatalf("instance after stop failure: running=%v stopCalls=%d", svc.running, svc.stopCalls)
	}
	if got := string(fs.written[binaryPath]); got != "old binary" {
		t.Fatalf("binary after stop failure = %q, want old binary", got)
	}
}

func TestUpgradeBackupFailureResumesRunningInstance(t *testing.T) {
	primary := errors.New("backup directory failed")
	fs := &upgradeTestFileSystem{
		fakeFileSystem: &fakeFileSystem{
			fileExists: map[string]bool{binaryPath: true},
			written:    map[string][]byte{binaryPath: []byte("old binary")},
		},
		mkdirErr: primary,
	}
	source := &fakeReleaseSource{}
	linkStorage(fs.fakeFileSystem, source)
	svc := &mockServiceManager{running: true}
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, source, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", noopProgress)
	if !errors.Is(err, primary) {
		t.Fatalf("Upgrade error = %v, want backup error", err)
	}
	if !svc.running {
		t.Fatal("backup failure should resume the running instance")
	}
	if got := string(fs.written[binaryPath]); got != "old binary" {
		t.Fatalf("binary after backup failure = %q, want old binary", got)
	}
}

func TestUpgradeReplacementFailureRestoresRunningInstance(t *testing.T) {
	primary := errors.New("replacement failed")
	fs := &upgradeTestFileSystem{
		fakeFileSystem: &fakeFileSystem{
			fileExists: map[string]bool{binaryPath: true},
			written:    map[string][]byte{binaryPath: []byte("old binary")},
		},
		failOnRename: 2,
		renameErr:    primary,
	}
	source := &fakeReleaseSource{}
	linkStorage(fs.fakeFileSystem, source)
	svc := &mockServiceManager{running: true}
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, source, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", noopProgress)
	if !errors.Is(err, primary) {
		t.Fatalf("Upgrade error = %v, want replacement error", err)
	}
	if got := string(fs.written[binaryPath]); got != "old binary" {
		t.Fatalf("binary after rollback = %q, want old binary", got)
	}
	if !svc.running {
		t.Fatal("rollback should restore the running instance")
	}
}

func TestUpgradeReplacementFailurePreservesStoppedInstance(t *testing.T) {
	primary := errors.New("replacement failed")
	fs := &upgradeTestFileSystem{
		fakeFileSystem: &fakeFileSystem{
			fileExists: map[string]bool{binaryPath: true},
			written:    map[string][]byte{binaryPath: []byte("old binary")},
		},
		failOnRename: 2,
		renameErr:    primary,
	}
	source := &fakeReleaseSource{}
	linkStorage(fs.fakeFileSystem, source)
	svc := &mockServiceManager{}
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, source, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", noopProgress)
	if !errors.Is(err, primary) {
		t.Fatalf("Upgrade error = %v, want replacement error", err)
	}
	if got := string(fs.written[binaryPath]); got != "old binary" {
		t.Fatalf("binary after rollback = %q, want old binary", got)
	}
	if svc.running || svc.startCalls != 0 {
		t.Fatalf("stopped instance after rollback: running=%v startCalls=%d", svc.running, svc.startCalls)
	}
}

func TestUpgradeStartFailureRestoresRunningInstance(t *testing.T) {
	primary := errors.New("start failed")
	fs := &fakeFileSystem{
		fileExists: map[string]bool{binaryPath: true},
		written:    map[string][]byte{binaryPath: []byte("old binary")},
	}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{running: true, startErrors: []error{primary, nil}}
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, source, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", noopProgress)
	if !errors.Is(err, primary) {
		t.Fatalf("Upgrade error = %v, want start error", err)
	}
	if got := string(fs.written[binaryPath]); got != "old binary" {
		t.Fatalf("binary after rollback = %q, want old binary", got)
	}
	if !svc.running {
		t.Fatal("rollback should restore the running instance")
	}
}

func TestUpgradeFailureReportsErrorProgress(t *testing.T) {
	primary := errors.New("start failed")
	fs := &fakeFileSystem{
		fileExists: map[string]bool{binaryPath: true},
		written:    map[string][]byte{binaryPath: []byte("old binary")},
	}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{running: true, startErrors: []error{primary, nil}}
	m := newLifecycleTestManager(fs, source, svc)
	var events []ProgressEvent

	if err := m.Upgrade(context.Background(), "v1.19.0", func(e ProgressEvent) {
		events = append(events, e)
	}); err == nil {
		t.Fatal("expected start failure")
	}
	for _, event := range events {
		if strings.HasPrefix(event.Message, "Running ") {
			t.Fatalf("events = %v, failure must not report success", events)
		}
		if event.Error != nil {
			return
		}
	}
	t.Fatalf("events = %v, want a failure progress event", events)
}

func TestUpgradeRollbackFailurePreservesPrimaryAndRecoveryErrors(t *testing.T) {
	primary := errors.New("start failed")
	recovery := errors.New("restore rename failed")
	fs := &upgradeTestFileSystem{
		fakeFileSystem: &fakeFileSystem{
			fileExists: map[string]bool{binaryPath: true},
			written:    map[string][]byte{binaryPath: []byte("old binary")},
		},
		failOnRename: 3,
		renameErr:    recovery,
	}
	source := &fakeReleaseSource{}
	linkStorage(fs.fakeFileSystem, source)
	svc := &mockServiceManager{running: true, startErrors: []error{primary}}
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, source, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", noopProgress)
	if !errors.Is(err, primary) || !errors.Is(err, recovery) {
		t.Fatalf("Upgrade error = %v, want primary and recovery errors", err)
	}
	if !strings.Contains(err.Error(), "rollback failed") || !strings.Contains(err.Error(), "manual recovery") {
		t.Fatalf("Upgrade error = %v, want rollback-failed manual-recovery diagnostic", err)
	}
}

func TestUpgradeCancellationBeforeStopLeavesInstanceUntouched(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fs := &fakeFileSystem{
		fileExists: map[string]bool{binaryPath: true},
		written:    map[string][]byte{binaryPath: []byte("old binary")},
	}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{running: true}
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, source, svc)

	err := m.Upgrade(ctx, "v1.19.0", noopProgress)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Upgrade error = %v, want cancellation", err)
	}
	if svc.stopCalls != 0 {
		t.Fatalf("stop calls = %d, want no stop before cancellation", svc.stopCalls)
	}
	if got := string(fs.written[binaryPath]); got != "old binary" {
		t.Fatalf("binary after cancellation = %q, want old binary", got)
	}
}

func TestUpgradeCancellationAfterReplacementRollsBack(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fs := &upgradeTestFileSystem{
		fakeFileSystem: &fakeFileSystem{
			fileExists: map[string]bool{binaryPath: true},
			written:    map[string][]byte{binaryPath: []byte("old binary")},
		},
		cancel: cancel,
	}
	source := &fakeReleaseSource{}
	linkStorage(fs.fakeFileSystem, source)
	svc := &mockServiceManager{running: true}
	m := NewLifecycleManager(fs, &fakeCmdRunner{}, source, svc)

	err := m.Upgrade(ctx, "v1.19.0", noopProgress)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Upgrade error = %v, want cancellation", err)
	}
	if got := string(fs.written[binaryPath]); got != "old binary" {
		t.Fatalf("binary after cancellation = %q, want old binary", got)
	}
	if !svc.running {
		t.Fatal("cancellation rollback should restore the running instance")
	}
	for path := range fs.written {
		if strings.Contains(path, ".tmp.") {
			t.Fatalf("temporary artifact remains after cancellation: %q", path)
		}
	}
}

func TestUpgradeReportsReplacementPhases(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{binaryPath: true}}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{running: true}
	m := newLifecycleTestManager(fs, source, svc)
	var phases []InstallationPhase

	if err := m.Upgrade(context.Background(), "v1.19.0", func(e ProgressEvent) {
		phases = append(phases, e.Phase)
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []InstallationPhase{PhaseUpgradeCheck, PhaseUpgradeFetch, PhaseUpgradeStop, PhaseUpgradeReplace, PhaseUpgradeStart}
	last := -1
	for _, phase := range want {
		found := -1
		for i := last + 1; i < len(phases); i++ {
			if phases[i] == phase {
				found = i
				break
			}
		}
		if found == -1 {
			t.Fatalf("phases = %v, want %v in order", phases, want)
		}
		last = found
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

func TestListVersionsRejectsEmptyResult(t *testing.T) {
	m := NewLifecycleManager(&fakeFileSystem{}, &fakeCmdRunner{}, &fakeReleaseSource{}, &mockServiceManager{})

	_, err := m.ListVersions(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no versions") {
		t.Fatalf("ListVersions error = %v, want empty-result error", err)
	}
}
