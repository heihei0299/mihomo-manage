package manager

import (
	"bytes"
	"compress/gzip"
	"context"
	"strings"
	"testing"
)

type mockSystem struct {
	fileExists     map[string]bool
	written        map[string][]byte
	writeErr       error
	removeErr      error
	renameErr      error
	cmdOutput      string
	cmdErr         error
	downloadErr    error
	downloadCalled bool
	versions       []VersionInfo
	versionsErr    error
	removed        []string
	renamed        map[string]string
}

func (m *mockSystem) Download(ctx context.Context, url, dest string) error {
	m.downloadCalled = true
	if m.downloadErr != nil {
		return m.downloadErr
	}
	if m.written == nil {
		m.written = make(map[string][]byte)
	}
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write([]byte("proxies:\n  - server: fetched-node\n"))
	gw.Close()
	m.written[dest] = buf.Bytes()
	return nil
}

func (m *mockSystem) FileExists(path string) bool {
	return m.fileExists[path]
}

func (m *mockSystem) ReadFile(path string) ([]byte, error) {
	if m.written == nil {
		return nil, assertError{"not found"}
	}
	data, ok := m.written[path]
	if !ok {
		return nil, assertError{"not found"}
	}
	return data, nil
}

func (m *mockSystem) WriteFile(path string, data []byte, perm uint32) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	if m.written == nil {
		m.written = make(map[string][]byte)
	}
	m.written[path] = data
	return nil
}

func (m *mockSystem) Remove(path string) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	m.removed = append(m.removed, path)
	return nil
}

func (m *mockSystem) Rename(oldPath, newPath string) error {
	if m.renameErr != nil {
		return m.renameErr
	}
	if m.renamed == nil {
		m.renamed = make(map[string]string)
	}
	m.renamed[oldPath] = newPath
	if m.fileExists == nil {
		m.fileExists = make(map[string]bool)
	}
	if m.written != nil {
		if data, ok := m.written[oldPath]; ok {
			m.written[newPath] = data
			delete(m.written, oldPath)
		}
	}
	m.fileExists[newPath] = true
	delete(m.fileExists, oldPath)
	return nil
}

func (m *mockSystem) MkdirAll(path string, perm uint32) error {
	return nil
}

func (m *mockSystem) Chmod(path string, perm uint32) error {
	return nil
}

func (m *mockSystem) RunCommand(name string, args ...string) (string, error) {
	return m.cmdOutput, m.cmdErr
}

func (m *mockSystem) ListVersions(ctx context.Context, owner, repo string, limit int) ([]VersionInfo, error) {
	return m.versions, m.versionsErr
}

func (m *mockSystem) LatestVersion(ctx context.Context, owner, repo string) (string, error) {
	if len(m.versions) > 0 {
		return m.versions[0].Tag, nil
	}
	return "v1.0.0", nil
}

type mockServiceManager struct {
	running      bool
	stopped      bool
	err          error
	registerErr  error
	startErr     error
	stopErr      error
	restartErr   error
	reloadErr    error
	registered   string
	reloadCalled bool
}

func (m *mockServiceManager) IsRunning(name string) (bool, error) {
	return m.running, m.err
}

func (m *mockServiceManager) Register(name, serviceFilePath string) error {
	if m.registerErr != nil {
		return m.registerErr
	}
	m.registered = name
	return nil
}

func (m *mockServiceManager) Unregister(name string) error {
	return nil
}

func (m *mockServiceManager) Start(name string) error {
	if m.startErr != nil {
		return m.startErr
	}
	m.running = true
	return nil
}

func (m *mockServiceManager) Stop(name string) error {
	if m.stopErr != nil {
		return m.stopErr
	}
	m.running = false
	m.stopped = true
	return nil
}

func (m *mockServiceManager) Restart(name string) error {
	if m.restartErr != nil {
		return m.restartErr
	}
	return nil
}

func (m *mockServiceManager) Reload(name string) error {
	m.reloadCalled = true
	if m.reloadErr != nil {
		return m.reloadErr
	}
	return nil
}

type assertError struct{ msg string }

func (e assertError) Error() string { return e.msg }

func TestStatusNotInstalled(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{}}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	status, err := m.Status(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.Installed {
		t.Errorf("expected Installed=false, got true")
	}
	if status.InstanceState != Stopped {
		t.Errorf("expected InstanceState=Stopped, got %v", status.InstanceState)
	}
	if status.Version != "" {
		t.Errorf("expected empty Version, got %q", status.Version)
	}
}

func TestStatusInstalledStopped(t *testing.T) {
	sys := &mockSystem{
		fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true},
		cmdOutput:  "Mihomo Meta v1.18.0 linux amd64",
	}
	svc := &mockServiceManager{running: false}
	m := New(sys, svc)

	status, err := m.Status(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Installed {
		t.Errorf("expected Installed=true, got false")
	}
	if status.InstanceState != Stopped {
		t.Errorf("expected InstanceState=Stopped, got %v", status.InstanceState)
	}
	if status.Version != "v1.18.0" {
		t.Errorf("expected Version=v1.18.0, got %q", status.Version)
	}
}

func TestStatusInstalledRunning(t *testing.T) {
	sys := &mockSystem{
		fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true},
		cmdOutput:  "Mihomo Meta v1.18.0 linux amd64",
	}
	svc := &mockServiceManager{running: true}
	m := New(sys, svc)

	status, err := m.Status(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !status.Installed {
		t.Errorf("expected Installed=true, got false")
	}
	if status.InstanceState != Running {
		t.Errorf("expected InstanceState=Running, got %v", status.InstanceState)
	}
}

func TestStatusServiceManagerError(t *testing.T) {
	sys := &mockSystem{
		fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true},
	}
	svc := &mockServiceManager{err: assertError{"service not found"}}
	m := New(sys, svc)

	_, err := m.Status(context.Background())

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestReleaseURLIncludesOSArch(t *testing.T) {
	url := releaseURL("linux", "amd64", "v1.19.27")

	if !strings.Contains(url, "linux") {
		t.Error("BUG 5: release URL should contain OS (linux)")
	}
	if !strings.Contains(url, "amd64") {
		t.Error("BUG 5: release URL should contain arch (amd64)")
	}
	if !strings.Contains(url, ".gz") {
		t.Error("BUG 5: release URL should have .gz extension")
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		output   string
		expected string
	}{
		{"Mihomo Meta v1.18.0 linux amd64", "v1.18.0"},
		{"mihomo v1.18.0", "v1.18.0"},
		{"Mihomo Meta V1.18.0 linux amd64 go1.22.0", "V1.18.0"},
		{"unknown output format", "unknown output format"},
		{"", ""},
	}

	for _, tt := range tests {
		sys := &mockSystem{cmdOutput: tt.output}
		got, err := parseVersion(sys, "/dummy")
		if err != nil {
			t.Errorf("parseVersion(%q) unexpected error: %v", tt.output, err)
			continue
		}
		if got != tt.expected {
			t.Errorf("parseVersion(%q) = %q, want %q", tt.output, got, tt.expected)
		}
	}
}

func TestParseVersionError(t *testing.T) {
	sys := &mockSystem{cmdErr: assertError{"command failed"}}
	_, err := parseVersion(sys, "/dummy")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestInstallDownloadFails(t *testing.T) {
	sys := &mockSystem{downloadErr: assertError{"network error"}}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	var events []ProgressEvent
	err := m.Install(context.Background(), "v1.18.0", func(e ProgressEvent) {
		events = append(events, e)
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(events) == 0 || events[0].Phase != PhaseFetch {
		t.Errorf("expected first event phase to be Fetch, got %v", events)
	}
}

func TestInstallHappyPath(t *testing.T) {
	sys := &mockSystem{}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	var phases []InstallationPhase
	var lastErr error
	err := m.Install(context.Background(), "v1.18.0", func(e ProgressEvent) {
		if e.Error == nil {
			phases = append(phases, e.Phase)
		}
		lastErr = e.Error
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lastErr != nil {
		t.Fatalf("last event had error: %v", lastErr)
	}
	if len(phases) == 0 {
		t.Fatal("expected at least one phase event")
	}
	last := phases[len(phases)-1]
	if last != PhaseStart {
		t.Errorf("expected last phase to be PhaseStart, got %v", last)
	}
}

func TestInstallCreatesServiceFile(t *testing.T) {
	sys := &mockSystem{}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	m.Install(context.Background(), "v1.18.0", func(e ProgressEvent) {})

	hasServiceFile := false
	for path := range sys.written {
		if strings.Contains(path, "mihomo.service") || strings.Contains(path, "systemd") {
			hasServiceFile = true
			break
		}
	}
	if !hasServiceFile {
		t.Error("BUG 1: Install did not write a systemd service file — systemctl enable will succeed silently but point at a non-existent unit")
	}
}

func TestInstallDeployFailsRollsBack(t *testing.T) {
	sys := &mockSystem{writeErr: assertError{"disk full"}}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	var events []ProgressEvent
	err := m.Install(context.Background(), "v1.18.0", func(e ProgressEvent) {
		events = append(events, e)
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRenderConfigBasicSubstitution(t *testing.T) {
	tmpl := `proxies:
{{subscription}}
rules:
{{routing_rules}}`
	sub := `  - name: node1
    type: ss
    server: example.com`
	rules := `DOMAIN-SUFFIX,google.com,Proxy`

	got, err := RenderConfig(tmpl, sub, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(got, sub) {
		t.Errorf("output should contain subscription data")
	}
	if !strings.Contains(got, rules) {
		t.Errorf("output should contain routing rules")
	}
	if strings.Contains(got, "{{subscription}}") {
		t.Errorf("output should not contain unsubstituted placeholder")
	}
	if strings.Contains(got, "{{routing_rules}}") {
		t.Errorf("output should not contain unsubstituted placeholder")
	}
}

func TestRenderConfigEmptySubscription(t *testing.T) {
	tmpl := `proxies: {{subscription}}`
	got, err := RenderConfig(tmpl, "", "rules: all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "proxies: " {
		t.Errorf("expected empty subscription, got %q", got)
	}
}

func TestRenderConfigEmptyRules(t *testing.T) {
	tmpl := `rules: {{routing_rules}}`
	got, err := RenderConfig(tmpl, "proxies: x", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "rules: " {
		t.Errorf("expected empty rules, got %q", got)
	}
}

func TestRenderConfigNoPlaceholders(t *testing.T) {
	tmpl := `static config`
	got, err := RenderConfig(tmpl, "sub", "rules")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != tmpl {
		t.Errorf("expected template unchanged, got %q", got)
	}
}

func TestSubscriptionRemoteURLFetched(t *testing.T) {
	sys := &mockSystem{
		fileExists: map[string]bool{
			ConfigTemplatePath:                         true,
			"/opt/mihomo-manager/state/subscription-url.txt": true,
		},
		written: map[string][]byte{
			ConfigTemplatePath: []byte(`proxies: {{subscription}}`),
			"/opt/mihomo-manager/state/subscription-url.txt": []byte(`https://example.com/sub`),
		},
	}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	m.UpdateConfig(context.Background())

	if !sys.downloadCalled {
		t.Error("BUG 2: subscription set with URL should trigger Download but it was never called — URL literal is substituted verbatim")
	}
}

func TestUpdateConfigHappyPath(t *testing.T) {
	sys := &mockSystem{
		fileExists: map[string]bool{
			"/opt/mihomo/etc/config-template.yaml":            true,
			"/opt/mihomo-manager/state/subscription-data.txt": true,
			"/opt/mihomo/etc/rules.txt":                       true,
		},
		written: map[string][]byte{
			"/opt/mihomo/etc/config-template.yaml": []byte(`proxies:
{{subscription}}
rules:
{{routing_rules}}`),
			"/opt/mihomo-manager/state/subscription-data.txt": []byte(`  - name: node1
    type: ss
    server: example.com`),
			"/opt/mihomo/etc/rules.txt": []byte(`DOMAIN-KEYWORD,google,Proxy`),
		},
	}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	preview, err := m.PreviewConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(preview, "DOMAIN-KEYWORD,google,Proxy") {
		t.Errorf("preview should contain routing rules")
	}
	if !strings.Contains(preview, "node1") {
		t.Errorf("preview should contain subscription data")
	}
}

func TestUpdateConfigReloadsInstance(t *testing.T) {
	sys := &mockSystem{
		fileExists: map[string]bool{
			"/opt/mihomo/etc/config-template.yaml": true,
		},
		written: map[string][]byte{
			"/opt/mihomo/etc/config-template.yaml": []byte(`test: {{subscription}}`),
		},
	}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	m.UpdateConfig(context.Background())

	if !svc.reloadCalled {
		t.Error("BUG 3: UpdateConfig should call Reload after writing config but it never did — new config sits unapplied on disk")
	}
}

func TestUpdateConfigCreatesBackup(t *testing.T) {
	sys := &mockSystem{
		fileExists: map[string]bool{
			"/opt/mihomo/etc/config-template.yaml": true,
			"/opt/mihomo/etc/config.yaml":          true,
		},
		written: map[string][]byte{
			"/opt/mihomo/etc/config-template.yaml": []byte(`test: {{subscription}}`),
			"/opt/mihomo/etc/config.yaml":          []byte(`old content`),
		},
	}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	err := m.UpdateConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasBackup := false
	for path := range sys.written {
		if strings.Contains(path, "config.yaml.bak.") {
			hasBackup = true
		}
	}
	if !hasBackup {
		t.Error("expected a backup file config.yaml.bak.<timestamp> to be created")
	}
}

func TestStartNotInstalled(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{}}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for not installed")
	}
}

func TestStartStopped(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	svc := &mockServiceManager{running: false}
	m := New(sys, svc)

	err := m.Start(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.running {
		t.Error("expected service to be running after Start")
	}
}

func TestStartAlreadyRunning(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	svc := &mockServiceManager{running: true}
	m := New(sys, svc)

	err := m.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for already running")
	}
}

func TestStopNotInstalled(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{}}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	err := m.Stop(context.Background())
	if err == nil {
		t.Fatal("expected error for not installed")
	}
}

func TestStopRunning(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	svc := &mockServiceManager{running: true}
	m := New(sys, svc)

	err := m.Stop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.running {
		t.Error("expected service to be stopped after Stop")
	}
}

func TestStopAlreadyStopped(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	svc := &mockServiceManager{running: false}
	m := New(sys, svc)

	err := m.Stop(context.Background())
	if err == nil {
		t.Fatal("expected error for already stopped")
	}
}

func TestRestartNotInstalled(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{}}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	err := m.Restart(context.Background())
	if err == nil {
		t.Fatal("expected error for not installed")
	}
}

func TestRestartRunning(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	svc := &mockServiceManager{running: true}
	m := New(sys, svc)

	err := m.Restart(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReloadNotInstalled(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{}}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	err := m.Reload(context.Background())
	if err == nil {
		t.Fatal("expected error for not installed")
	}
}

func TestReloadRunning(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	svc := &mockServiceManager{running: true}
	m := New(sys, svc)

	err := m.Reload(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReloadStopped(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	svc := &mockServiceManager{running: false}
	m := New(sys, svc)

	err := m.Reload(context.Background())
	if err == nil {
		t.Fatal("expected error for reload when stopped")
	}
}

func TestUpgradeDownloadFails(t *testing.T) {
	sys := &mockSystem{
		fileExists:  map[string]bool{"/opt/mihomo/bin/mihomo": true},
		downloadErr: assertError{"network error"},
	}
	svc := &mockServiceManager{running: true}
	m := New(sys, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", func(e ProgressEvent) {})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestUpgradeHappyPath(t *testing.T) {
	sys := &mockSystem{
		fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true},
	}
	svc := &mockServiceManager{running: true}
	m := New(sys, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", func(e ProgressEvent) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpgradeStartFailsRollsBack(t *testing.T) {
	sys := &mockSystem{
		fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true},
	}
	svc := &mockServiceManager{running: true, startErr: assertError{"start failed"}}
	m := New(sys, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", func(e ProgressEvent) {})
	if err == nil {
		t.Fatal("expected error due to start failure")
	}
}

func TestUpgradeNotInstalled(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{}}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	err := m.Upgrade(context.Background(), "v1.19.0", func(e ProgressEvent) {})
	if err == nil {
		t.Fatal("expected error for not installed")
	}
}

func TestListVersions(t *testing.T) {
	sys := &mockSystem{
		versions: []VersionInfo{
			{Tag: "v1.19.0"},
			{Tag: "v1.18.0"},
			{Tag: "v1.17.0"},
		},
	}
	svc := &mockServiceManager{}
	m := New(sys, svc)

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

func TestUninstallNotInstalled(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{}}
	svc := &mockServiceManager{}
	m := New(sys, svc)

	err := m.Uninstall(context.Background(), false, func(e ProgressEvent) {})
	if err == nil {
		t.Fatal("expected error for not installed")
	}
}

func TestUninstallCleanup(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	svc := &mockServiceManager{running: true}
	m := New(sys, svc)

	err := m.Uninstall(context.Background(), false, func(e ProgressEvent) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.stopped {
		t.Error("expected service to be stopped")
	}
}

func TestUninstallKeepBackup(t *testing.T) {
	sys := &mockSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	svc := &mockServiceManager{running: true}
	m := New(sys, svc)

	err := m.Uninstall(context.Background(), true, func(e ProgressEvent) {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
