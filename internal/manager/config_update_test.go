package manager

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeDownloader is a GitHubReleases mock that writes plain text (not gzip)
// so we can verify subscription data appears in the final config.
type fakeDownloader struct {
	fakeGitHubReleases
	content string
}

func (m *fakeDownloader) Download(ctx context.Context, url, dest string) error {
	m.downloadCalled = true
	if m.downloadErr != nil {
		return m.downloadErr
	}
	if m.written == nil {
		m.written = make(map[string][]byte)
	}
	// Write plain text, not gzip — simulates a subscription URL response
	m.written[dest] = []byte(m.content)
	return nil
}

func TestRemoteSubscriptionDataAppearsInConfig(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			OverrideFilePath:    true,
			subscriptionURLFile: true,
		},
		written: map[string][]byte{
			OverrideFilePath: []byte(`proxy-groups:
  - name: Proxy
    type: select
rules:
  - MATCH,DIRECT`),
			subscriptionURLFile: []byte(`https://example.com/sub`),
		},
	}
	dl := &fakeDownloader{content: "proxies:\n  - name: node1\n    type: ss\n    server: example.com\n    port: 443"}
	linkStorage(fs, &dl.fakeGitHubReleases)

	m := NewConfigManager(fs, dl, nil, nil)

	err := m.UpdateConfig(context.Background())
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Check downloaded data was written to subscriptionDataFile
	subData, err := fs.ReadFile(subscriptionDataFile)
	if err != nil {
		t.Fatalf("subscriptionDataFile not written: %v", err)
	}
	if !strings.Contains(string(subData), "node1") {
		t.Errorf("subscriptionDataFile should contain downloaded proxy data, got: %s", string(subData))
	}

	// Check final config contains merged content
	configData, err := fs.ReadFile(configYAML)
	if err != nil {
		t.Fatalf("configYAML not written: %v", err)
	}
	cfg := string(configData)

	if !strings.Contains(cfg, "node1") {
		t.Errorf("config should contain subscription data (node1), got: %s", cfg)
	}
	if !strings.Contains(cfg, "Proxy") {
		t.Errorf("config should contain template proxy-groups, got: %s", cfg)
	}
	if !strings.Contains(cfg, "MATCH,DIRECT") {
		t.Errorf("config should contain template rules, got: %s", cfg)
	}
}

func TestLocalSubscriptionDataAppearsInConfig(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			OverrideFilePath: true,
		},
		written: map[string][]byte{
			OverrideFilePath:     []byte("proxy-groups:\n  - name: Proxy\n    type: select\n"),
			subscriptionDataFile: []byte("proxies:\n  - name: local-node\n    type: ss\n    server: local.example.com\n"),
		},
	}
	gh := &fakeGitHubReleases{}
	m := NewConfigManager(fs, gh, nil, nil)

	preview, err := m.PreviewConfig(context.Background())
	if err != nil {
		t.Fatalf("PreviewConfig failed: %v", err)
	}

	if !strings.Contains(preview, "local-node") {
		t.Errorf("preview should contain local subscription data, got: %q", preview)
	}
	if !strings.Contains(preview, "Proxy") {
		t.Errorf("preview should contain template proxy-groups, got: %q", preview)
	}
}

func TestSubscriptionWithTopLevelKeysViaUpdate(t *testing.T) {
	// Simulate: subscription data contains top-level keys (complete config)
	// Template overlays supplement missing fields only
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			OverrideFilePath:    true,
			subscriptionURLFile: true,
		},
		written: map[string][]byte{
			OverrideFilePath:    []byte("log-level: debug\n"),
			subscriptionURLFile: []byte(`https://example.com/sub`),
		},
	}
	dl := &fakeDownloader{content: "port: 7890\nmode: rule\nproxies:\n  - name: node1\n    type: ss\n    server: example.com"}
	linkStorage(fs, &dl.fakeGitHubReleases)

	m := NewConfigManager(fs, dl, nil, nil)

	// First update — this should download and write subscription data
	err := m.UpdateConfig(context.Background())
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Now preview should contain the downloaded data
	preview, err := m.PreviewConfig(context.Background())
	if err != nil {
		t.Fatalf("PreviewConfig after UpdateConfig failed: %v", err)
	}

	if !strings.Contains(preview, "node1") {
		t.Errorf("preview should contain subscription data (node1), got: %q", preview)
	}

	// Subscription port should be kept (template does not override)
	if !strings.Contains(preview, "port: 7890") {
		t.Errorf("preview should contain subscription port, got: %q", preview)
	}

	// Template log-level should supplement (not in subscription)
	if !strings.Contains(preview, "log-level: debug") {
		t.Errorf("preview should contain template log-level, got: %q", preview)
	}
}

func TestUpdateConfigWithValidatorPassesSubscriptionData(t *testing.T) {
	// Full flow with a no-op validator
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			OverrideFilePath:    true,
			subscriptionURLFile: true,
			configYAML:          true,
		},
		written: map[string][]byte{
			OverrideFilePath:    []byte("log-level: debug\n"),
			subscriptionURLFile: []byte(`https://example.com/sub`),
			configYAML:          []byte(`old config`),
		},
	}
	dl := &fakeDownloader{content: "proxies:\n  - name: fetched-node\n    type: ss\n    server: example.com"}
	linkStorage(fs, &dl.fakeGitHubReleases)

	m := NewConfigManager(fs, dl, &passValidator{}, nil)

	err := m.UpdateConfig(context.Background())
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Verify the final config contains the subscription data
	cfg, _ := fs.ReadFile(configYAML)
	if !strings.Contains(string(cfg), "fetched-node") {
		t.Errorf("config should contain downloaded subscription data, got: %s", string(cfg))
	}
}

func TestPipelineMergeOverridesBaseScalar(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true},
		written: map[string][]byte{
			OverrideFilePath:     []byte("port: 8888\nmode: global\n"),
			subscriptionDataFile: []byte("port: 7890\nmode: rule\nsocks-port: 7891\n"),
		},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	preview, err := m.PreviewConfig(context.Background())
	if err != nil {
		t.Fatalf("PreviewConfig failed: %v", err)
	}

	if !strings.Contains(preview, "port: 8888") {
		t.Errorf("template port should override base port: %s", preview)
	}
	if strings.Contains(preview, "port: 7890") {
		t.Errorf("base port should be replaced: %s", preview)
	}
	if !strings.Contains(preview, "mode: global") {
		t.Errorf("template mode should override base mode: %s", preview)
	}
}

func TestPipelineMergeAppendsArrays(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true},
		written: map[string][]byte{
			OverrideFilePath: []byte(`proxy-groups:
  - name: Custom
    type: select
rules:
  - MATCH,REJECT`),
			subscriptionDataFile: []byte(`proxies:
  - name: node1
    type: ss
proxy-groups:
  - name: Proxy
    type: select
rules:
  - DOMAIN-SUFFIX,example.com,Proxy`),
		},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	preview, err := m.PreviewConfig(context.Background())
	if err != nil {
		t.Fatalf("PreviewConfig failed: %v", err)
	}

	if !strings.Contains(preview, "node1") {
		t.Errorf("base proxy should be present: %s", preview)
	}
	if !strings.Contains(preview, "Proxy") {
		t.Errorf("base proxy-group should be present: %s", preview)
	}
	if !strings.Contains(preview, "Custom") {
		t.Errorf("template proxy-group should be appended: %s", preview)
	}
	if !strings.Contains(preview, "example.com") {
		t.Errorf("base rule should be present: %s", preview)
	}
	if !strings.Contains(preview, "REJECT") {
		t.Errorf("template rule should be appended: %s", preview)
	}
}

func TestPipelineMergeOverridesNonAppendArrays(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true},
		written: map[string][]byte{
			OverrideFilePath:     []byte("listen:\n  - 0.0.0.0:8080\n"),
			subscriptionDataFile: []byte("listen:\n  - 0.0.0.0:9090\n  - 127.0.0.1:9091\n"),
		},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	preview, err := m.PreviewConfig(context.Background())
	if err != nil {
		t.Fatalf("PreviewConfig failed: %v", err)
	}

	if !strings.Contains(preview, "8080") {
		t.Errorf("template listen should override (not in append list): %s", preview)
	}
	if strings.Contains(preview, "9090") {
		t.Errorf("base listen values should be replaced: %s", preview)
	}
	if strings.Contains(preview, "9091") {
		t.Errorf("base listen values should be replaced: %s", preview)
	}
}

func TestOldTemplatePlaceholderWarning(t *testing.T) {
	var warned string
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true},
		written: map[string][]byte{
			OverrideFilePath: []byte(`proxies: {{subscription}}`),
		},
	}
	p := newConfigPipeline(fs, &fakeGitHubReleases{}, ConfigPipelineOptions{
		Warn: func(msg string) { warned = msg },
	})

	_, err := p.Preview(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(warned, "old placeholder") {
		t.Errorf("expected deprecation warning, got: %q", warned)
	}
}

func TestPipelineMigratesLegacyTemplate(t *testing.T) {
	var warned string
	fs := &fakeFileSystem{
		fileExists: map[string]bool{legacyTemplatePath: true},
		written:    map[string][]byte{legacyTemplatePath: []byte("port: 8888\n")},
	}
	newConfigPipeline(fs, &fakeGitHubReleases{}, ConfigPipelineOptions{Warn: func(msg string) { warned = msg }})

	if !fs.FileExists(OverrideFilePath) {
		t.Errorf("legacy template should be migrated to %s", OverrideFilePath)
	}
	if fs.FileExists(legacyTemplatePath) {
		t.Errorf("legacy template should no longer exist after migration")
	}
	data, err := fs.ReadFile(OverrideFilePath)
	if err != nil || string(data) != "port: 8888\n" {
		t.Errorf("migrated override should carry legacy content, got %q err %v", data, err)
	}
	if !strings.Contains(warned, "migrat") {
		t.Errorf("expected migration warning, got: %q", warned)
	}
}

func TestPipelineMigrationSkipsWhenOverrideExists(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{legacyTemplatePath: true, OverrideFilePath: true},
		written:    map[string][]byte{OverrideFilePath: []byte("port: 9999\n")},
	}
	newConfigPipeline(fs, &fakeGitHubReleases{}, ConfigPipelineOptions{})

	if len(fs.renamed) != 0 {
		t.Errorf("no rename should happen when override already exists, got: %v", fs.renamed)
	}
}

func TestPipelineMigrationNoopWhenNothingExists(t *testing.T) {
	fs := &fakeFileSystem{}
	newConfigPipeline(fs, &fakeGitHubReleases{}, ConfigPipelineOptions{})
	if len(fs.renamed) != 0 {
		t.Errorf("no rename should happen when neither file exists, got: %v", fs.renamed)
	}
}

func TestPipelinePureSubscriptionWithoutOverride(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{subscriptionDataFile: true},
		written:    map[string][]byte{subscriptionDataFile: []byte("port: 7890\nmode: rule\n")},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	preview, err := m.PreviewConfig(context.Background())
	if err != nil {
		t.Fatalf("PreviewConfig failed: %v", err)
	}
	if !strings.Contains(preview, "port: 7890") {
		t.Errorf("pure subscription should pass through without override file, got: %s", preview)
	}

	if err := m.UpdateConfig(context.Background()); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}
	if fs.FileExists(OverrideFilePath) {
		t.Errorf("Apply should not auto-create the override file (pure-subscription mode)")
	}
}

func TestSetRemoteSubscriptionSelectsRemoteSourceAndClearsLocalState(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{subscriptionDataFile: true},
		written:    map[string][]byte{subscriptionDataFile: []byte("local: true\n")},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	if err := m.SetSubscriptionSource(context.Background(), "https://example.com/sub.yaml"); err != nil {
		t.Fatalf("SetSubscriptionSource failed: %v", err)
	}

	if got := string(fs.written[stateDir+"/subscription-source.txt"]); got != "remote\n" {
		t.Fatalf("source marker = %q, want remote", got)
	}
	if got := string(fs.written[subscriptionURLFile]); got != "https://example.com/sub.yaml" {
		t.Fatalf("subscription URL = %q", got)
	}
	removedData := false
	for _, path := range fs.removed {
		if path == subscriptionDataFile {
			removedData = true
			break
		}
	}
	if !removedData {
		t.Fatalf("removed files = %v, want local subscription data removed", fs.removed)
	}
}

func TestSetSubscriptionSourceRollsBackWhenMarkerWriteFails(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			subscriptionSourceFile: true,
			subscriptionDataFile:   true,
		},
		written: map[string][]byte{
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
		},
		writeErrByPath: map[string]error{
			subscriptionSourceFile: errors.New("marker write failed"),
		},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	if err := m.SetSubscriptionSource(context.Background(), "https://example.com/sub.yaml"); err == nil {
		t.Fatal("SetSubscriptionSource should report marker write failure")
	}
	if got := string(fs.written[subscriptionSourceFile]); got != "local\n" {
		t.Fatalf("source marker after rollback = %q, want local", got)
	}
	if got := string(fs.written[subscriptionDataFile]); got != "mode: rule\n" {
		t.Fatalf("subscription data after rollback = %q", got)
	}
	if _, exists := fs.written[subscriptionURLFile]; exists {
		t.Fatal("remote URL should not remain after rollback")
	}
}

func TestSetLocalSubscriptionSelectsLocalSourceAndClearsRemoteState(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{subscriptionURLFile: true},
		written:    map[string][]byte{subscriptionURLFile: []byte("https://example.com/sub.yaml")},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	const localData = "proxies:\n  - name: local\n"
	if err := m.SetSubscriptionSource(context.Background(), localData); err != nil {
		t.Fatalf("SetSubscriptionSource failed: %v", err)
	}

	if got := string(fs.written[subscriptionSourceFile]); got != "local\n" {
		t.Fatalf("source marker = %q, want local", got)
	}
	if got := string(fs.written[subscriptionDataFile]); got != localData {
		t.Fatalf("subscription data = %q", got)
	}
	removedURL := false
	for _, path := range fs.removed {
		if path == subscriptionURLFile {
			removedURL = true
			break
		}
	}
	if !removedURL {
		t.Fatalf("removed files = %v, want remote URL removed", fs.removed)
	}
}

func TestPreviewMigratesSingleLegacyRemoteSource(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{subscriptionURLFile: true},
		written:    map[string][]byte{subscriptionURLFile: []byte("https://example.com/sub.yaml")},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	if _, err := m.PreviewConfig(context.Background()); err != nil {
		t.Fatalf("PreviewConfig failed: %v", err)
	}
	if got := string(fs.written[subscriptionSourceFile]); got != "remote\n" {
		t.Fatalf("migrated source marker = %q, want remote", got)
	}
}

func TestPreviewRejectsConflictingLegacySources(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{subscriptionURLFile: true, subscriptionDataFile: true},
		written: map[string][]byte{
			subscriptionURLFile:  []byte("https://example.com/sub.yaml"),
			subscriptionDataFile: []byte("mode: rule\n"),
		},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	_, err := m.PreviewConfig(context.Background())
	if err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("PreviewConfig error = %v, want conflicting legacy source error", err)
	}
}

func TestLocalSourceDoesNotUseStaleRemoteURL(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			subscriptionURLFile: true,
			configYAML:          true,
		},
		written: map[string][]byte{
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
			subscriptionURLFile:    []byte("https://stale.example/sub.yaml"),
			configYAML:             []byte("mode: rule\n"),
		},
	}
	gh := &fakeGitHubReleases{}
	m := NewConfigManager(fs, gh, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}
	if gh.downloadCalled {
		t.Fatal("local source should not download the stale remote URL")
	}
}

func TestInvalidSubscriptionSourceIsRejected(t *testing.T) {
	fs := &fakeFileSystem{
		written: map[string][]byte{subscriptionSourceFile: []byte("unknown\n")},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	_, err := m.PreviewConfig(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid subscription source") {
		t.Fatalf("PreviewConfig error = %v, want invalid source error", err)
	}
}

func TestSetSubscriptionSourceRejectsEmptySource(t *testing.T) {
	m := NewConfigManager(&fakeFileSystem{}, &fakeGitHubReleases{}, &passValidator{}, nil)

	if err := m.SetSubscriptionSource(context.Background(), "  \n"); err == nil {
		t.Fatal("SetSubscriptionSource should reject an empty source")
	}
}

func TestUpdateConfigRejectsUnconfiguredSource(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true},
		written:    map[string][]byte{OverrideFilePath: []byte("mode: rule\n")},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil || !strings.Contains(err.Error(), "subscription source is not configured") {
		t.Fatalf("UpdateConfig error = %v, want unconfigured source error", err)
	}
	status, err := m.LastConfigApply(context.Background())
	if err != nil {
		t.Fatalf("LastConfigApply failed: %v", err)
	}
	if status.State != ConfigApplyFailed {
		t.Fatalf("status = %+v, want apply-failed", status)
	}
}

func TestRemoteSourceRejectsEmptyURLInsteadOfUsingCachedData(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			subscriptionURLFile:  true,
			subscriptionDataFile: true,
			OverrideFilePath:     true,
		},
		written: map[string][]byte{
			subscriptionSourceFile: []byte("remote\n"),
			subscriptionURLFile:    []byte("  \n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
			OverrideFilePath:       []byte("log-level: info\n"),
		},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil || !strings.Contains(err.Error(), "URL is empty") {
		t.Fatalf("UpdateConfig error = %v, want empty URL error", err)
	}
}

type recordingConfigValidator struct {
	path string
	err  error
}

type blockingSubscriptionDownloader struct {
	fakeGitHubReleases
	started chan struct{}
}

func (d *blockingSubscriptionDownloader) Download(ctx context.Context, url, dest string) error {
	close(d.started)
	<-ctx.Done()
	return ctx.Err()
}

type blockingConfigValidator struct {
	started chan struct{}
}

func (v *blockingConfigValidator) Validate(ctx context.Context, path string) error {
	close(v.started)
	<-ctx.Done()
	return ctx.Err()
}

func (v *recordingConfigValidator) Validate(ctx context.Context, path string) error {
	v.path = path
	return v.err
}

type fakeConfigUpdateLock struct {
	err      error
	acquired bool
	released bool
}

func (l *fakeConfigUpdateLock) Acquire(context.Context) (func(), error) {
	l.acquired = true
	if l.err != nil {
		return nil, l.err
	}
	return func() { l.released = true }, nil
}

func TestUpdateConfigDownloadHonorsCancellation(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true, subscriptionSourceFile: true, subscriptionURLFile: true},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("remote\n"),
			subscriptionURLFile:    []byte("https://example.com/sub.yaml"),
		},
	}
	dl := &blockingSubscriptionDownloader{started: make(chan struct{})}
	m := NewConfigManager(fs, dl, &passValidator{}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- m.UpdateConfig(ctx) }()
	<-dl.started
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("UpdateConfig error = %v, want cancellation", err)
	}
	requireConfigApplyState(t, m, ConfigApplyFailed)
}

func TestUpdateConfigValidationHonorsCancellation(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true, subscriptionSourceFile: true, subscriptionDataFile: true},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
		},
	}
	validator := &blockingConfigValidator{started: make(chan struct{})}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, validator, nil)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- m.UpdateConfig(ctx) }()
	<-validator.started
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("UpdateConfig error = %v, want cancellation", err)
	}
	status, err := m.LastConfigApply(context.Background())
	if err != nil {
		t.Fatalf("LastConfigApply failed: %v", err)
	}
	if status.State != ConfigApplyFailed {
		t.Fatalf("status = %+v, want apply-failed", status)
	}
}

func TestUpdateConfigReturnsBusyWhenLockUnavailable(t *testing.T) {
	lock := &fakeConfigUpdateLock{err: ErrConfigUpdateBusy}
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
		},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil, WithConfigUpdateLock(lock))

	if err := m.UpdateConfig(context.Background()); !errors.Is(err, ErrConfigUpdateBusy) {
		t.Fatalf("UpdateConfig error = %v, want busy error", err)
	}
	if !lock.acquired {
		t.Fatal("UpdateConfig should attempt to acquire the update lock")
	}
	if _, exists := fs.written[configYAML]; exists {
		t.Fatal("busy update should not write generated config")
	}
}

func TestUpdateConfigReleasesLockAfterSuccess(t *testing.T) {
	lock := &fakeConfigUpdateLock{}
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
		},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil, WithConfigUpdateLock(lock))

	if err := m.UpdateConfig(context.Background()); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}
	if !lock.released {
		t.Fatal("UpdateConfig should release the update lock")
	}
}

func localApplyTestFileSystem() *fakeFileSystem {
	return &fakeFileSystem{
		fileExists: map[string]bool{
			OverrideFilePath:       true,
			subscriptionSourceFile: true,
			subscriptionDataFile:   true,
		},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
		},
	}
}

func requireConfigApplyState(t *testing.T, m ConfigManager, want ConfigApplyState) {
	t.Helper()
	status, err := m.LastConfigApply(context.Background())
	if err != nil {
		t.Fatalf("LastConfigApply failed: %v", err)
	}
	if status.State != want {
		t.Fatalf("status = %+v, want %s", status, want)
	}
}

func TestUpdateConfigDownloadFailureRecordsApplyStatus(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			OverrideFilePath:       true,
			subscriptionSourceFile: true,
			subscriptionURLFile:    true,
		},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("remote\n"),
			subscriptionURLFile:    []byte("https://example.com/sub.yaml"),
		},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{downloadErr: errors.New("download failed")}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report download failure")
	}
	requireConfigApplyState(t, m, ConfigApplyFailed)
}

func TestUpdateConfigCleansDownloadTempAfterFailure(t *testing.T) {
	const tempPath = subscriptionDataFile + ".tmp"
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			OverrideFilePath:       true,
			subscriptionSourceFile: true,
			subscriptionURLFile:    true,
			tempPath:               true,
		},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("remote\n"),
			subscriptionURLFile:    []byte("https://example.com/sub.yaml"),
			tempPath:               []byte("stale download"),
		},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{downloadErr: errors.New("download failed")}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report download failure")
	}
	if _, ok := fs.written[tempPath]; ok {
		t.Fatal("download temp file should be removed after failure")
	}
	requireConfigApplyState(t, m, ConfigApplyFailed)
}

func TestUpdateConfigStagingDirectoryFailureRecordsApplyStatus(t *testing.T) {
	fs := localApplyTestFileSystem()
	fs.mkdirErrFunc = func(path string) error {
		if strings.Contains(path, ".mihomo-config-staging-") {
			return errors.New("staging directory unavailable")
		}
		return nil
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report staging directory failure")
	}
	requireConfigApplyState(t, m, ConfigApplyFailed)
}

func TestUpdateConfigStagedWriteFailureRecordsApplyStatus(t *testing.T) {
	fs := localApplyTestFileSystem()
	fs.writeErrFunc = func(path string) error {
		if strings.Contains(path, ".mihomo-config-staging-") {
			return errors.New("staged config is not writable")
		}
		return nil
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report staged write failure")
	}
	requireConfigApplyState(t, m, ConfigApplyFailed)
}

func TestUpdateConfigBackupFailureRecordsApplyStatus(t *testing.T) {
	fs := localApplyTestFileSystem()
	fs.fileExists[configYAML] = true
	fs.written[configYAML] = []byte("old\n")
	fs.writeErrFunc = func(path string) error {
		if strings.HasPrefix(path, configYAML+".bak.") {
			return errors.New("backup is not writable")
		}
		return nil
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report backup failure")
	}
	requireConfigApplyState(t, m, ConfigApplyFailed)
}

func TestUpdateConfigRenameFailureRecordsApplyStatus(t *testing.T) {
	fs := localApplyTestFileSystem()
	fs.renameErrByPath = map[string]error{configYAML: errors.New("rename failed")}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report rename failure")
	}
	requireConfigApplyState(t, m, ConfigApplyFailed)
}

func TestUpdateConfigStagingCleanupFailureRecordsApplyStatus(t *testing.T) {
	fs := localApplyTestFileSystem()
	fs.removeErrFunc = func(path string) error {
		if strings.Contains(path, ".mihomo-config-staging-") {
			return errors.New("staging cleanup failed")
		}
		return nil
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report staging cleanup failure")
	}
	requireConfigApplyState(t, m, ConfigApplyFailed)
}

func TestUpdateConfigValidatesStagedConfigBeforeAtomicCommit(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			OverrideFilePath:       true,
			subscriptionSourceFile: true,
			subscriptionDataFile:   true,
			configYAML:             true,
		},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
			configYAML:             []byte("old\n"),
		},
	}
	validator := &recordingConfigValidator{}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, validator, nil)

	if err := m.UpdateConfig(context.Background()); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}
	if validator.path == configYAML || !strings.HasSuffix(validator.path, "/config.yaml") {
		t.Fatalf("validator path = %q, want staged config.yaml", validator.path)
	}
	committed := false
	for source, destination := range fs.renamed {
		if destination == configYAML && source != configYAML {
			committed = true
		}
	}
	if !committed {
		t.Fatalf("renames = %v, want staged config atomically committed", fs.renamed)
	}
}

func TestUpdateConfigRecordsAppliedStatus(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true, subscriptionSourceFile: true, subscriptionDataFile: true},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
		},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}
	status, err := m.LastConfigApply(context.Background())
	if err != nil {
		t.Fatalf("LastConfigApply failed: %v", err)
	}
	if status.State != ConfigApplied || status.ConfigHash == "" || status.AttemptedAt.IsZero() {
		t.Fatalf("status = %+v, want applied status with timestamp and hash", status)
	}
}

func TestUpdateConfigValidationFailureRecordsStatusAndPreservesConfig(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true, subscriptionSourceFile: true, subscriptionDataFile: true, configYAML: true},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
			configYAML:             []byte("old\n"),
		},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &failValidator{err: errors.New("invalid staged config")}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should fail validation")
	}
	if got := string(fs.written[configYAML]); got != "old\n" {
		t.Fatalf("config after validation failure = %q, want old config", got)
	}
	status, err := m.LastConfigApply(context.Background())
	if err != nil {
		t.Fatalf("LastConfigApply failed: %v", err)
	}
	if status.State != ConfigValidationFailed || !strings.Contains(status.ErrorSummary, "invalid staged config") {
		t.Fatalf("status = %+v, want validation-failed with error", status)
	}
}

func TestUpdateConfigReloadFailureRecordsPendingStatus(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true, subscriptionSourceFile: true, subscriptionDataFile: true, configYAML: true},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
			configYAML:             []byte("old\n"),
		},
	}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, func(context.Context) error {
		return errors.New("reload failed")
	})

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should fail when reload fails")
	}
	if got := string(fs.written[configYAML]); got == "old\n" {
		t.Fatal("validated generated config should remain after reload failure")
	}
	status, err := m.LastConfigApply(context.Background())
	if err != nil {
		t.Fatalf("LastConfigApply failed: %v", err)
	}
	if status.State != ConfigPendingReload || !strings.Contains(status.ErrorSummary, "reload failed") {
		t.Fatalf("status = %+v, want pending-reload with error", status)
	}
}

func TestLastConfigApplyReportsCorruptStateAsUnknown(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{configApplyStatusFile: []byte("not json")}}
	m := NewConfigManager(fs, &fakeGitHubReleases{}, &passValidator{}, nil)

	status, err := m.LastConfigApply(context.Background())
	if err != nil {
		t.Fatalf("LastConfigApply failed: %v", err)
	}
	if status.State != ConfigUnknown || status.ErrorSummary == "" {
		t.Fatalf("status = %+v, want unknown with diagnostic", status)
	}
}

// passValidator is a ConfigValidator that always passes
type passValidator struct{}

func (v *passValidator) Validate(ctx context.Context, configPath string) error {
	return nil
}
