package manager

import (
	"context"
	"strings"
	"testing"
)

// fakeDownloader is a ReleaseSource mock that writes plain text (not gzip)
// so we can verify subscription data appears in the final config.
type fakeDownloader struct {
	fakeReleaseSource
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
	linkStorage(fs, &dl.fakeReleaseSource)

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
	source := &fakeReleaseSource{}
	m := NewConfigManager(fs, source, nil, nil)

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
	linkStorage(fs, &dl.fakeReleaseSource)

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
	linkStorage(fs, &dl.fakeReleaseSource)

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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

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

func TestPipelinePureSubscriptionWithoutOverride(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{subscriptionDataFile: true},
		written:    map[string][]byte{subscriptionDataFile: []byte("port: 7890\nmode: rule\n")},
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

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

// passValidator is a ConfigValidator that always passes
type passValidator struct{}

func (v *passValidator) Validate(ctx context.Context, configPath string) error {
	return nil
}
