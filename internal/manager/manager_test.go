package manager

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
)

type fakeFileSystem struct {
	fileExists map[string]bool
	written    map[string][]byte
	removed    []string
	renamed    map[string]string
}

func (m *fakeFileSystem) FileExists(path string) bool {
	return m.fileExists[path]
}

func (m *fakeFileSystem) ReadFile(path string) ([]byte, error) {
	if m.written == nil {
		return nil, os.ErrNotExist
	}
	data, ok := m.written[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return data, nil
}

func (m *fakeFileSystem) WriteFile(path string, data []byte, perm uint32) error {
	if m.written == nil {
		m.written = make(map[string][]byte)
	}
	m.written[path] = data
	if m.fileExists == nil {
		m.fileExists = make(map[string]bool)
	}
	m.fileExists[path] = true
	return nil
}

func (m *fakeFileSystem) Remove(path string) error {
	m.removed = append(m.removed, path)
	for existing := range m.written {
		if existing == path || strings.HasPrefix(existing, path+"/") {
			delete(m.written, existing)
		}
	}
	for existing := range m.fileExists {
		if existing == path || strings.HasPrefix(existing, path+"/") {
			delete(m.fileExists, existing)
		}
	}
	return nil
}

func (m *fakeFileSystem) Rename(oldPath, newPath string) error {
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

func (m *fakeFileSystem) MkdirAll(path string, perm uint32) error {
	return nil
}

func (m *fakeFileSystem) Chmod(path string, perm uint32) error {
	return nil
}

type fakeCmdRunner struct {
	cmdOutput string
	cmdErr    error
}

func (m *fakeCmdRunner) RunCommand(ctx context.Context, name string, args ...string) (string, error) {
	return m.cmdOutput, m.cmdErr
}

func (m *fakeCmdRunner) RunCommandIgnoreExit(ctx context.Context, name string, args ...string) (string, error) {
	return m.cmdOutput, m.cmdErr
}

type fakeReleaseSource struct {
	downloadErr      error
	downloadCalled   bool
	expectedChecksum string
	checksumErr      error
	checksumCalled   bool
	downloadData     []byte
	written          map[string][]byte
	versions         []VersionInfo
	versionsErr      error
	latestVersion    string
	latestErr        error
	latestCalls      int
}

func fakeReleaseArchive() []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	gw.Write([]byte("proxies:\n  - server: fetched-node\n"))
	gw.Close()
	return buf.Bytes()
}

func (m *fakeReleaseSource) Download(ctx context.Context, url, dest string) error {
	m.downloadCalled = true
	if m.downloadErr != nil {
		return m.downloadErr
	}
	if m.written == nil {
		m.written = make(map[string][]byte)
	}
	if m.downloadData != nil {
		m.written[dest] = m.downloadData
	} else {
		m.written[dest] = fakeReleaseArchive()
	}
	return nil
}

// linkStorage makes source share the same written map as fs, so
// Download writes are visible to ReadFile.
func linkStorage(fs *fakeFileSystem, source *fakeReleaseSource) {
	if fs.written == nil {
		fs.written = make(map[string][]byte)
	}
	source.written = fs.written
}

func (m *fakeReleaseSource) ExpectedChecksum(ctx context.Context, owner, repo, version, assetName string) (string, error) {
	m.checksumCalled = true
	if m.checksumErr != nil {
		return "", m.checksumErr
	}
	if m.expectedChecksum != "" {
		return m.expectedChecksum, nil
	}
	data := fakeReleaseArchive()
	if m.downloadData != nil {
		data = m.downloadData
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func (m *fakeReleaseSource) ListVersions(ctx context.Context, owner, repo string, limit int) ([]VersionInfo, error) {
	return m.versions, m.versionsErr
}

func (m *fakeReleaseSource) LatestVersion(ctx context.Context, owner, repo string) (string, error) {
	m.latestCalls++
	if m.latestErr != nil {
		return "", m.latestErr
	}
	if m.latestVersion != "" {
		return m.latestVersion, nil
	}
	if len(m.versions) > 0 {
		return m.versions[0].Tag, nil
	}
	return "v1.0.0", nil
}

type mockServiceManager struct {
	running          bool
	stopped          bool
	err              error
	registerErr      error
	startErr         error
	startDoesNotRun  bool
	runningStates    []bool
	runningCalls     int
	stopErr          error
	restartErr       error
	reloadErr        error
	registered       string
	reloadCalled     bool
	autoStartEnabled bool
}

func (m *mockServiceManager) IsRunning(ctx context.Context, name string) (bool, error) {
	m.runningCalls++
	if len(m.runningStates) > 0 {
		m.running = m.runningStates[0]
		m.runningStates = m.runningStates[1:]
	}
	return m.running, m.err
}

func (m *mockServiceManager) Register(ctx context.Context, name, serviceFilePath string) error {
	if m.registerErr != nil {
		return m.registerErr
	}
	m.registered = name
	return nil
}

func (m *mockServiceManager) Unregister(ctx context.Context, name string) error {
	return nil
}

func (m *mockServiceManager) Start(ctx context.Context, name string) error {
	if m.startErr != nil {
		return m.startErr
	}
	if !m.startDoesNotRun {
		m.running = true
	}
	return nil
}

func (m *mockServiceManager) Stop(ctx context.Context, name string) error {
	if m.stopErr != nil {
		return m.stopErr
	}
	m.running = false
	m.stopped = true
	return nil
}

func (m *mockServiceManager) Restart(ctx context.Context, name string) error {
	if m.restartErr != nil {
		return m.restartErr
	}
	return nil
}

func (m *mockServiceManager) Reload(ctx context.Context, name string) error {
	m.reloadCalled = true
	if m.reloadErr != nil {
		return m.reloadErr
	}
	return nil
}

func (m *mockServiceManager) EnableAutoStart(ctx context.Context, name, serviceFilePath string) error {
	m.autoStartEnabled = true
	return nil
}

func (m *mockServiceManager) DisableAutoStart(ctx context.Context, name string) error {
	m.autoStartEnabled = false
	return nil
}

func (m *mockServiceManager) AutoStartEnabled(ctx context.Context, name string) (bool, error) {
	return m.autoStartEnabled, nil
}

type testManager struct {
	fs     *fakeFileSystem
	cmd    *fakeCmdRunner
	source *fakeReleaseSource
	svc    *mockServiceManager
	ctrl   ServiceControl
	life   LifecycleManager
	cfg    ConfigManager
	sched  ScheduleManager
}

func newTestManager() *testManager {
	fs := &fakeFileSystem{}
	cmd := &fakeCmdRunner{}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	svc := &mockServiceManager{}
	return &testManager{
		fs:     fs,
		cmd:    cmd,
		source: source,
		svc:    svc,
		ctrl:   NewServiceControl(fs, cmd, svc, passConfigValidation),
		life:   NewLifecycleManager(fs, cmd, source, svc),
		cfg:    NewConfigManager(fs, source, &configValidator{}, func(ctx context.Context) error { return svc.Reload(ctx, serviceName) }),
		sched:  NewScheduleManagerWithPlatform(fs, &fakePlatformScheduler{}, "/opt/mihomo-manager/bin/mihomo-manager"),
	}
}

type testError struct{ msg string }

func (e testError) Error() string { return e.msg }

func passConfigValidation(context.Context) error { return nil }

type orderedServiceManager struct {
	*mockServiceManager
	events *[]string
}

func (m *orderedServiceManager) Start(ctx context.Context, name string) error {
	*m.events = append(*m.events, "start")
	return m.mockServiceManager.Start(ctx, name)
}

func (m *orderedServiceManager) Restart(ctx context.Context, name string) error {
	*m.events = append(*m.events, "restart")
	return m.mockServiceManager.Restart(ctx, name)
}

func TestStartValidationFailurePreventsServiceControl(t *testing.T) {
	validationErr := errors.New("config invalid")
	events := []string{}
	m := NewServiceControl(
		&fakeFileSystem{fileExists: map[string]bool{binaryPath: true}},
		&fakeCmdRunner{},
		&orderedServiceManager{
			mockServiceManager: &mockServiceManager{running: false},
			events:             &events,
		},
		func(context.Context) error {
			events = append(events, "validate")
			return validationErr
		},
	)

	err := m.Start(context.Background())
	if !errors.Is(err, validationErr) {
		t.Fatalf("Start error = %v, want validation error", err)
	}
	if len(events) != 1 || events[0] != "validate" {
		t.Fatalf("events = %v, want only validation", events)
	}
}

func TestRestartValidationFailurePreventsServiceControl(t *testing.T) {
	validationErr := errors.New("config invalid")
	events := []string{}
	m := NewServiceControl(
		&fakeFileSystem{fileExists: map[string]bool{binaryPath: true}},
		&fakeCmdRunner{},
		&orderedServiceManager{
			mockServiceManager: &mockServiceManager{running: true},
			events:             &events,
		},
		func(context.Context) error {
			events = append(events, "validate")
			return validationErr
		},
	)

	err := m.Restart(context.Background())
	if !errors.Is(err, validationErr) {
		t.Fatalf("Restart error = %v, want validation error", err)
	}
	if len(events) != 1 || events[0] != "validate" {
		t.Fatalf("events = %v, want only validation", events)
	}
}

func TestStartValidatesBeforeCallingServiceControl(t *testing.T) {
	events := []string{}
	m := NewServiceControl(
		&fakeFileSystem{fileExists: map[string]bool{binaryPath: true}},
		&fakeCmdRunner{},
		&orderedServiceManager{
			mockServiceManager: &mockServiceManager{running: false},
			events:             &events,
		},
		func(context.Context) error {
			events = append(events, "validate")
			return nil
		},
	)

	if err := m.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if len(events) != 2 || events[0] != "validate" || events[1] != "start" {
		t.Fatalf("events = %v, want [validate start]", events)
	}
}

func TestRestartValidatesBeforeCallingServiceControl(t *testing.T) {
	events := []string{}
	m := NewServiceControl(
		&fakeFileSystem{fileExists: map[string]bool{binaryPath: true}},
		&fakeCmdRunner{},
		&orderedServiceManager{
			mockServiceManager: &mockServiceManager{running: true},
			events:             &events,
		},
		func(context.Context) error {
			events = append(events, "validate")
			return nil
		},
	)

	if err := m.Restart(context.Background()); err != nil {
		t.Fatalf("Restart failed: %v", err)
	}
	if len(events) != 2 || events[0] != "validate" || events[1] != "restart" {
		t.Fatalf("events = %v, want [validate restart]", events)
	}
}

func TestStartAndRestartPropagateServiceErrors(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		want := errors.New("start failed")
		m := NewServiceControl(
			&fakeFileSystem{fileExists: map[string]bool{binaryPath: true}},
			&fakeCmdRunner{},
			&mockServiceManager{startErr: want},
			passConfigValidation,
		)

		if err := m.Start(context.Background()); !errors.Is(err, want) {
			t.Fatalf("Start error = %v, want %v", err, want)
		}
	})

	t.Run("restart", func(t *testing.T) {
		want := errors.New("restart failed")
		m := NewServiceControl(
			&fakeFileSystem{fileExists: map[string]bool{binaryPath: true}},
			&fakeCmdRunner{},
			&mockServiceManager{restartErr: want},
			passConfigValidation,
		)

		if err := m.Restart(context.Background()); !errors.Is(err, want) {
			t.Fatalf("Restart error = %v, want %v", err, want)
		}
	})
}

func TestStatusNotInstalled(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{}
	m := NewServiceControl(fs, cmd, svc, passConfigValidation)

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
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{cmdOutput: "Mihomo Meta v1.18.0 linux amd64"}
	svc := &mockServiceManager{running: false}
	m := NewServiceControl(fs, cmd, svc, passConfigValidation)

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
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{cmdOutput: "Mihomo Meta v1.18.0 linux amd64"}
	svc := &mockServiceManager{running: true}
	m := NewServiceControl(fs, cmd, svc, passConfigValidation)

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
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{err: testError{"service not found"}}
	m := NewServiceControl(fs, cmd, svc, passConfigValidation)

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
		cmd := &fakeCmdRunner{cmdOutput: tt.output}
		got, err := parseVersion(context.Background(), cmd, "/dummy")
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
	cmd := &fakeCmdRunner{cmdErr: testError{"command failed"}}
	_, err := parseVersion(context.Background(), cmd, "/dummy")
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestSetSubscriptionSourceNoDeadWrite(t *testing.T) {
	fs := &fakeFileSystem{}
	source := &fakeReleaseSource{}
	m := NewConfigManager(fs, source, &configValidator{}, nil)

	err := m.SetSubscriptionSource(context.Background(), "https://example.com/sub")
	if err != nil {
		t.Fatalf("SetSubscriptionSource failed: %v", err)
	}

	if _, wroteData := fs.written[subscriptionDataFile]; wroteData {
		t.Error("SetSubscriptionSource should not write to subscriptionDataFile — it's dead code, only subscriptionURLFile should be written")
	}
	if _, wroteURL := fs.written[subscriptionURLFile]; !wroteURL {
		t.Error("SetSubscriptionSource should write to subscriptionURLFile")
	}
}

func TestSubscriptionRemoteURLFetched(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			OverrideFilePath: true,
			"/opt/mihomo-manager/state/subscription-url.txt": true,
		},
		written: map[string][]byte{
			OverrideFilePath: []byte(`proxies: {{subscription}}`),
			"/opt/mihomo-manager/state/subscription-url.txt": []byte(`https://example.com/sub`),
		},
	}
	source := &fakeReleaseSource{}
	linkStorage(fs, source)
	m := NewConfigManager(fs, source, &configValidator{}, nil)

	m.UpdateConfig(context.Background())

	if !source.downloadCalled {
		t.Error("BUG 2: subscription set with URL should trigger Download but it was never called — URL literal is substituted verbatim")
	}
}

func TestPreviewConfigMissingSubscriptionFile(t *testing.T) {
	fs := &fakeFileSystem{
		written: map[string][]byte{
			OverrideFilePath: []byte("socks-port: 7891\n"),
		},
	}
	source := &fakeReleaseSource{}
	m := NewConfigManager(fs, source, &configValidator{}, nil)

	result, err := m.PreviewConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "socks-port: 7891") {
		t.Errorf("expected template content in result, got %q", result)
	}
}

func TestUpdateConfigEmptyURL(t *testing.T) {
	fs := &fakeFileSystem{
		written: map[string][]byte{
			OverrideFilePath:       []byte(`test: {{subscription}}`),
			subscriptionSourceFile: []byte("remote\n"),
			subscriptionURLFile:    []byte(``),
		},
	}
	source := &fakeReleaseSource{}
	m := NewConfigManager(fs, source, &configValidator{}, nil)

	err := m.UpdateConfig(context.Background())
	if err == nil || !strings.Contains(err.Error(), "URL is empty") {
		t.Fatalf("error = %v, want empty URL error", err)
	}
}

func TestUpdateConfigNoExistingConfig(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			OverrideFilePath: true,
		},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
		},
	}
	source := &fakeReleaseSource{}
	m := NewConfigManager(fs, source, &passValidator{}, nil)

	err := m.UpdateConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should succeed without creating a backup (no existing configYAML)
}

func TestUpdateConfigHappyPath(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			OverrideFilePath:     true,
			subscriptionDataFile: true,
		},
		written: map[string][]byte{
			OverrideFilePath: []byte(`proxy-groups:
  - name: Proxy
    type: select
rules:
  - MATCH,DIRECT`),
			subscriptionDataFile: []byte(`proxies:
  - name: node1
    type: ss
    server: example.com`),
		},
	}
	source := &fakeReleaseSource{}
	m := NewConfigManager(fs, source, &configValidator{}, nil)

	preview, err := m.PreviewConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(preview, "MATCH,DIRECT") {
		t.Errorf("preview should contain rules from template")
	}
	if !strings.Contains(preview, "node1") {
		t.Errorf("preview should contain subscription data")
	}
	if !strings.Contains(preview, "Proxy") {
		t.Errorf("preview should contain template proxy-groups")
	}
}

func TestUpdateConfigReloadsInstance(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			"/opt/mihomo/etc/config-template.yaml": true,
		},
		written: map[string][]byte{
			"/opt/mihomo/etc/config-template.yaml": []byte("mode: rule\n"),
			subscriptionSourceFile:                 []byte("local\n"),
			subscriptionDataFile:                   []byte("mode: rule\n"),
		},
	}
	source := &fakeReleaseSource{}
	svc := &mockServiceManager{}
	m := NewConfigManager(fs, source, &passValidator{}, func(ctx context.Context) error {
		return svc.Reload(context.Background(), serviceName)
	})

	m.UpdateConfig(context.Background())

	if !svc.reloadCalled {
		t.Error("BUG 3: UpdateConfig should call Reload after writing config but it never did — new config sits unapplied on disk")
	}
}

func TestUpdateConfigCreatesBackup(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			"/opt/mihomo/etc/config-template.yaml": true,
			"/opt/mihomo/etc/config.yaml":          true,
		},
		written: map[string][]byte{
			"/opt/mihomo/etc/config-template.yaml": []byte("mode: rule\n"),
			"/opt/mihomo/etc/config.yaml":          []byte(`old content`),
			subscriptionSourceFile:                 []byte("local\n"),
			subscriptionDataFile:                   []byte("mode: rule\n"),
		},
	}
	source := &fakeReleaseSource{}
	m := NewConfigManager(fs, source, &passValidator{}, nil)

	err := m.UpdateConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasBackup := false
	for path := range fs.written {
		if strings.Contains(path, "config.yaml.bak.") {
			hasBackup = true
		}
	}
	if !hasBackup {
		t.Error("expected a backup file config.yaml.bak.<timestamp> to be created")
	}
}

func TestStartNotInstalled(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{}
	m := NewServiceControl(fs, cmd, svc, passConfigValidation)

	err := m.Start(context.Background())
	if !errors.Is(err, ErrMihomoNotInstalled) {
		t.Fatalf("error = %v, want ErrMihomoNotInstalled", err)
	}
}

func TestStartStopped(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{running: false}
	m := NewServiceControl(fs, cmd, svc, passConfigValidation)

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
	m := NewServiceControl(fs, cmd, svc, passConfigValidation)

	err := m.Start(context.Background())
	if !errors.Is(err, ErrMihomoAlreadyRunning) {
		t.Fatalf("error = %v, want ErrMihomoAlreadyRunning", err)
	}
}

func TestStopNotInstalled(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{}
	m := NewServiceControl(fs, cmd, svc, passConfigValidation)

	err := m.Stop(context.Background())
	if !errors.Is(err, ErrMihomoNotInstalled) {
		t.Fatalf("error = %v, want ErrMihomoNotInstalled", err)
	}
}

func TestStopRunning(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{running: true}
	m := NewServiceControl(fs, cmd, svc, passConfigValidation)

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
	m := NewServiceControl(fs, cmd, svc, passConfigValidation)

	err := m.Stop(context.Background())
	if !errors.Is(err, ErrMihomoNotRunning) {
		t.Fatalf("error = %v, want ErrMihomoNotRunning", err)
	}
}

func TestRestartNotInstalled(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{}
	m := NewServiceControl(fs, cmd, svc, passConfigValidation)

	err := m.Restart(context.Background())
	if !errors.Is(err, ErrMihomoNotInstalled) {
		t.Fatalf("error = %v, want ErrMihomoNotInstalled", err)
	}
}

func TestRestartRunning(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{running: true}
	m := NewServiceControl(fs, cmd, svc, passConfigValidation)

	err := m.Restart(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReloadNotInstalled(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{}
	m := NewServiceControl(fs, cmd, svc, passConfigValidation)

	err := m.Reload(context.Background())
	if !errors.Is(err, ErrMihomoNotInstalled) {
		t.Fatalf("error = %v, want ErrMihomoNotInstalled", err)
	}
}

func TestReloadRunning(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{running: true}
	m := NewServiceControl(fs, cmd, svc, passConfigValidation)

	err := m.Reload(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReloadStopped(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{"/opt/mihomo/bin/mihomo": true}}
	cmd := &fakeCmdRunner{}
	svc := &mockServiceManager{running: false}
	m := NewServiceControl(fs, cmd, svc, passConfigValidation)

	err := m.Reload(context.Background())
	if !errors.Is(err, ErrMihomoNotRunning) {
		t.Fatalf("error = %v, want ErrMihomoNotRunning", err)
	}
}
