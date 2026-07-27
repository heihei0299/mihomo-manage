package config

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/anomalyco/mihomo-manager/internal/domain"
)

func TestRenderConfigBasicSubstitution(t *testing.T) {
	tmpl := `proxies:
{{subscription}}
rules:
{{routing_rules}}`
	sub := `  - name: node1
     type: ss
     server: example.com`
	rules := `DOMAIN-SUFFIX,google.com,Proxy`

	got, err := renderConfig(tmpl, sub, rules)
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
	got, err := renderConfig(tmpl, "", "rules: all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "proxies: " {
		t.Errorf("expected empty subscription, got %q", got)
	}
}

func TestRenderConfigEmptyRules(t *testing.T) {
	tmpl := `rules: {{routing_rules}}`
	got, err := renderConfig(tmpl, "proxies: x", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "rules: " {
		t.Errorf("expected empty rules, got %q", got)
	}
}

func TestRenderConfigNoPlaceholders(t *testing.T) {
	tmpl := `static config`
	got, err := renderConfig(tmpl, "sub", "rules")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != tmpl {
		t.Errorf("expected template unchanged, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

type fakeFileSystem struct {
	fileExists  map[string]bool
	written     map[string][]byte
	writeErr    error
	removeErr   error
	renameErr   error
	removed     []string
	renamed     map[string]string
	readFileErr map[string]error
}

func (m *fakeFileSystem) FileExists(path string) bool {
	return m.fileExists[path]
}

func (m *fakeFileSystem) ReadFile(path string) ([]byte, error) {
	if m.readFileErr != nil {
		if err, ok := m.readFileErr[path]; ok {
			return nil, err
		}
	}
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
	if m.writeErr != nil {
		return m.writeErr
	}
	if m.written == nil {
		m.written = make(map[string][]byte)
	}
	m.written[path] = data
	return nil
}

func (m *fakeFileSystem) Remove(path string) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	m.removed = append(m.removed, path)
	return nil
}

func (m *fakeFileSystem) Rename(oldPath, newPath string) error {
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

func (m *fakeCmdRunner) RunCommand(name string, args ...string) (string, error) {
	return m.cmdOutput, m.cmdErr
}

func (m *fakeCmdRunner) RunCommandIgnoreExit(name string, args ...string) (string, error) {
	return m.cmdOutput, m.cmdErr
}

type fakeGitHubReleases struct {
	downloadErr    error
	downloadCalled bool
	written        map[string][]byte
	versions       []domain.VersionInfo
	versionsErr    error
}

func (m *fakeGitHubReleases) Download(ctx context.Context, url, dest string) error {
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

func (m *fakeGitHubReleases) ListVersions(ctx context.Context, owner, repo string, limit int) ([]domain.VersionInfo, error) {
	return m.versions, m.versionsErr
}

func (m *fakeGitHubReleases) LatestVersion(ctx context.Context, owner, repo string) (string, error) {
	if len(m.versions) > 0 {
		return m.versions[0].Tag, nil
	}
	return "v1.0.0", nil
}

// linkStorage makes gh share the same written map as fs, so
// Download writes are visible to ReadFile.
func linkStorage(fs *fakeFileSystem, gh *fakeGitHubReleases) {
	if fs.written == nil {
		fs.written = make(map[string][]byte)
	}
	gh.written = fs.written
}

type mockServiceManager struct {
	running          bool
	stopped          bool
	err              error
	registerErr      error
	startErr         error
	stopErr          error
	restartErr       error
	reloadErr        error
	registered       string
	reloadCalled     bool
	autoStartEnabled bool
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

func (m *mockServiceManager) EnableAutoStart(name, serviceFilePath string) error {
	m.autoStartEnabled = true
	return nil
}

func (m *mockServiceManager) DisableAutoStart(name string) error {
	m.autoStartEnabled = false
	return nil
}

func (m *mockServiceManager) AutoStartEnabled(name string) (bool, error) {
	return m.autoStartEnabled, nil
}

type testError struct{ msg string }

func (e testError) Error() string { return e.msg }

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

// ---------------------------------------------------------------------------
// Config pipeline tests
// ---------------------------------------------------------------------------

func TestUpdateConfigReadURLError(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			ConfigTemplatePath: true,
			SubscriptionURLFile: true,
		},
		written: map[string][]byte{
			ConfigTemplatePath: []byte(`test: {{subscription}}`),
		},
		readFileErr: map[string]error{SubscriptionURLFile: testError{"permission denied"}},
	}
	gh := &fakeGitHubReleases{}
	cmd := &fakeCmdRunner{}
	m := NewManager(fs, gh, NewValidator(cmd), nil)

	err := m.UpdateConfig(context.Background())
	if err == nil {
		t.Error("expected error when ReadFile fails on SubscriptionURLFile")
	}
}

func TestSetSubscriptionSourceNoDeadWrite(t *testing.T) {
	fs := &fakeFileSystem{}
	gh := &fakeGitHubReleases{}
	m := NewManager(fs, gh, nil, nil)

	err := m.SetSubscriptionSource(context.Background(), "https://example.com/sub")
	if err != nil {
		t.Fatalf("SetSubscriptionSource failed: %v", err)
	}

	if _, wroteData := fs.written[SubscriptionDataFile]; wroteData {
		t.Error("SetSubscriptionSource should not write to SubscriptionDataFile — it's dead code, only SubscriptionURLFile should be written")
	}
	if _, wroteURL := fs.written[SubscriptionURLFile]; !wroteURL {
		t.Error("SetSubscriptionSource should write to SubscriptionURLFile")
	}
}

func TestSubscriptionRemoteURLFetched(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			ConfigTemplatePath:                  true,
			"/opt/mihomo-manager/state/subscription-url.txt": true,
		},
		written: map[string][]byte{
			ConfigTemplatePath:                  []byte(`proxies: {{subscription}}`),
			"/opt/mihomo-manager/state/subscription-url.txt": []byte(`https://example.com/sub`),
		},
	}
	gh := &fakeGitHubReleases{}
	linkStorage(fs, gh)
	cmd := &fakeCmdRunner{}
	m := NewManager(fs, gh, NewValidator(cmd), nil)

	m.UpdateConfig(context.Background())

	if !gh.downloadCalled {
		t.Error("BUG 2: subscription set with URL should trigger Download but it was never called — URL literal is substituted verbatim")
	}
}

func TestPreviewConfigSubscriptionReadError(t *testing.T) {
	fs := &fakeFileSystem{
		written: map[string][]byte{
			ConfigTemplatePath: []byte(`test: {{subscription}}`),
		},
		readFileErr: map[string]error{SubscriptionDataFile: testError{"permission denied"}},
	}
	gh := &fakeGitHubReleases{}
	m := NewManager(fs, gh, nil, nil)

	_, err := m.PreviewConfig(context.Background())
	if err == nil {
		t.Error("expected error when ReadFile fails on SubscriptionDataFile with non-ErrNotExist error")
	}
}

func TestPreviewConfigRulesReadError(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			ConfigTemplatePath: true,
		},
		written: map[string][]byte{
			ConfigTemplatePath: []byte(`test`),
		},
		readFileErr: map[string]error{RoutingRulesPath: testError{"permission denied"}},
	}
	gh := &fakeGitHubReleases{}
	m := NewManager(fs, gh, nil, nil)

	_, err := m.PreviewConfig(context.Background())
	if err == nil {
		t.Error("expected error when ReadFile fails on RoutingRulesPath with non-ErrNotExist error")
	}
}

func TestPreviewConfigMissingSubscriptionFile(t *testing.T) {
	fs := &fakeFileSystem{
		written: map[string][]byte{
			ConfigTemplatePath: []byte(`proxies: {{subscription}}`),
		},
	}
	gh := &fakeGitHubReleases{}
	m := NewManager(fs, gh, nil, nil)

	result, err := m.PreviewConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "proxies: " {
		t.Errorf("expected empty subscription data, got %q", result)
	}
}

func TestUpdateConfigEmptyURL(t *testing.T) {
	fs := &fakeFileSystem{
		written: map[string][]byte{
			ConfigTemplatePath:  []byte(`test: {{subscription}}`),
			SubscriptionURLFile: []byte(``),
		},
	}
	gh := &fakeGitHubReleases{}
	cmd := &fakeCmdRunner{}
	m := NewManager(fs, gh, NewValidator(cmd), nil)

	err := m.UpdateConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateConfigNoExistingConfig(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			ConfigTemplatePath: true,
		},
		written: map[string][]byte{
			ConfigTemplatePath: []byte(`test: {{subscription}}`),
		},
	}
	gh := &fakeGitHubReleases{}
	cmd := &fakeCmdRunner{}
	m := NewManager(fs, gh, NewValidator(cmd), nil)

	err := m.UpdateConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should succeed without creating a backup (no existing ConfigYAML)
}

func TestUpdateConfigHappyPath(t *testing.T) {
	fs := &fakeFileSystem{
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
	gh := &fakeGitHubReleases{}
	cmd := &fakeCmdRunner{}
	m := NewManager(fs, gh, NewValidator(cmd), nil)

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
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			"/opt/mihomo/etc/config-template.yaml": true,
		},
		written: map[string][]byte{
			"/opt/mihomo/etc/config-template.yaml": []byte(`test: {{subscription}}`),
		},
	}
	gh := &fakeGitHubReleases{}
	svc := &mockServiceManager{}
	cmd := &fakeCmdRunner{}
	m := NewManager(fs, gh, NewValidator(cmd), func(ctx context.Context) error {
		return svc.Reload(ServiceName)
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
			"/opt/mihomo/etc/config-template.yaml": []byte(`test: {{subscription}}`),
			"/opt/mihomo/etc/config.yaml":          []byte(`old content`),
		},
	}
	gh := &fakeGitHubReleases{}
	cmd := &fakeCmdRunner{}
	m := NewManager(fs, gh, NewValidator(cmd), nil)

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

func TestLifecycleSubscriptionUpdate(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			ConfigTemplatePath:  true,
			SubscriptionURLFile: true,
			ConfigYAML:          true,
		},
		written: map[string][]byte{
			ConfigTemplatePath:  []byte(`proxies: {{subscription}}`),
			SubscriptionURLFile: []byte(`https://example.com/sub`),
			ConfigYAML:          []byte(`old config`),
		},
	}
	gh := &fakeGitHubReleases{}
	linkStorage(fs, gh)
	svc := &mockServiceManager{}
	cmd := &fakeCmdRunner{}
	m := NewManager(fs, gh, NewValidator(cmd), func(ctx context.Context) error {
		return svc.Reload(ServiceName)
	})

	err := m.UpdateConfig(context.Background())
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	assertFileExists(t, fs, ConfigYAML, "config should be updated")
	if !gh.downloadCalled {
		t.Error("BUG 2: remote subscription URL should have been fetched via Download")
	}
	if !svc.reloadCalled {
		t.Error("BUG 3: UpdateConfig should reload service after writing config")
	}
}
