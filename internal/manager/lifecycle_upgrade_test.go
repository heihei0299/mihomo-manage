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
