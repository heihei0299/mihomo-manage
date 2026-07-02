package manager

import (
	"context"
	"testing"
	"time"
)

func noopProgress(ProgressEvent) {}

func TestLifecycleInstall(t *testing.T) {
	sys := &mockSystem{}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	err := m.Install(context.Background(), "v1.18.0", noopProgress)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	assertFileExists(t, sys, binaryPath, "binary should be deployed")
	assertFileExists(t, sys, ConfigTemplatePath, "template should be created")
	assertFileExists(t, sys, configYAML, "config should be created")
	assertFileExists(t, sys, defaultServiceUnitPath, "BUG 1: service unit file should be created")
	if !svc.running {
		t.Error("service should be running after Install")
	}
}

func TestLifecycleInstallThenStatus(t *testing.T) {
	sys := &mockSystem{cmdOutput: "Mihomo Meta v1.18.0 linux amd64"}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	m.Install(context.Background(), "v1.18.0", noopProgress)

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
	sys := &mockSystem{
		fileExists: map[string]bool{
			ConfigTemplatePath:                           true,
			subscriptionDataFile:                     true,
			configYAML:                               true,
		},
		written: map[string][]byte{
			ConfigTemplatePath:       []byte(`proxies: {{subscription}}`),
			subscriptionDataFile: []byte(`https://example.com/sub`),
			configYAML:           []byte(`old config`),
		},
		downloadErr: nil,
	}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	err := m.UpdateConfig(context.Background())
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	assertFileExists(t, sys, configYAML, "config should be updated")
	if !sys.downloadCalled {
		t.Error("BUG 2: remote subscription URL should have been fetched via Download")
	}
	if !svc.reloadCalled {
		t.Error("BUG 3: UpdateConfig should reload service after writing config")
	}
}

func TestLifecycleInstallRollbackOnDeployFail(t *testing.T) {
	sys := &mockSystem{
		writeErr: assertError{"disk full"},
	}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	err := m.Install(context.Background(), "v1.18.0", noopProgress)
	if err == nil {
		t.Fatal("expected Install to fail")
	}

	if svc.running {
		t.Error("service should not be running after failed install")
	}
}

func TestScheduleSetAndStop(t *testing.T) {
	sys := &mockSystem{
		fileExists: map[string]bool{
			ConfigTemplatePath: true,
		},
		written: map[string][]byte{
			ConfigTemplatePath: []byte(`test: {{subscription}}`),
		},
	}
	svc := &mockServiceManager{}
	m := New(sys, svc)

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
	sys := &mockSystem{}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	err := m.SetSchedule(context.Background(), time.Minute)
	if err == nil {
		t.Fatal("expected error for interval < 1h")
	}
}

func assertFileExists(t *testing.T, sys *mockSystem, path string, msg string) {
	t.Helper()
	if sys.written != nil {
		if _, ok := sys.written[path]; ok {
			return
		}
	}
	for _, newPath := range sys.renamed {
		if newPath == path {
			return
		}
	}
	t.Errorf("%s: expected %q to exist (written or renamed)", msg, path)
}
