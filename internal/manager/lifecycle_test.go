package manager

import (
	"context"
	"testing"
	"time"
)

func noopProgress(ProgressEvent) {}

func TestLifecycleInstall(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{}
	linkStorage(fs, gh)
	svc := &mockServiceManager{}
	m := New(fs, cmd, gh, svc)

	err := m.Install(context.Background(), "v1.18.0", true, noopProgress)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	assertFileExists(t, fs, binaryPath, "binary should be deployed")
	assertFileExists(t, fs, ConfigTemplatePath, "template should be created")
	assertFileExists(t, fs, configYAML, "config should be created")
	assertFileExists(t, fs, defaultServiceUnitPath, "BUG 1: service unit file should be created")
	if !svc.running {
		t.Error("service should be running after Install")
	}
}

func TestLifecycleInstallThenStatus(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &fakeCmdRunner{cmdOutput: "Mihomo Meta v1.18.0 linux amd64"}
	gh := &fakeGitHubReleases{}
	linkStorage(fs, gh)
	svc := &mockServiceManager{}
	m := New(fs, cmd, gh, svc)

	m.Install(context.Background(), "v1.18.0", true, noopProgress)

	status, err := m.Status(context.Background())
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
			ConfigTemplatePath:  true,
			subscriptionURLFile: true,
			configYAML:          true,
		},
		written: map[string][]byte{
			ConfigTemplatePath:  []byte(`proxies: {{subscription}}`),
			subscriptionURLFile: []byte(`https://example.com/sub`),
			configYAML:          []byte(`old config`),
		},
	}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{}
	linkStorage(fs, gh)
	svc := &mockServiceManager{}
	m := New(fs, cmd, gh, svc)

	err := m.UpdateConfig(context.Background())
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	assertFileExists(t, fs, configYAML, "config should be updated")
	if !gh.downloadCalled {
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
	gh := &fakeGitHubReleases{}
	linkStorage(fs, gh)
	svc := &mockServiceManager{}
	m := New(fs, cmd, gh, svc)

	err := m.Install(context.Background(), "v1.18.0", true, noopProgress)
	if err == nil {
		t.Fatal("expected Install to fail")
	}

	if svc.running {
		t.Error("service should not be running after failed install")
	}
}

func TestScheduleSetAndStop(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			ConfigTemplatePath: true,
		},
		written: map[string][]byte{
			ConfigTemplatePath: []byte(`test: {{subscription}}`),
		},
	}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{}
	svc := &mockServiceManager{}
	m := New(fs, cmd, gh, svc)

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
	fs := &fakeFileSystem{}
	cmd := &fakeCmdRunner{}
	gh := &fakeGitHubReleases{}
	svc := &mockServiceManager{}
	m := New(fs, cmd, gh, svc)

	err := m.SetSchedule(context.Background(), time.Minute)
	if err == nil {
		t.Fatal("expected error for interval < 1h")
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
