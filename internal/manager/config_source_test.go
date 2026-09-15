package manager

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestOldTemplatePlaceholderWarning(t *testing.T) {
	var warned string
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true},
		written: map[string][]byte{
			OverrideFilePath: []byte(`proxies: {{subscription}}`),
		},
	}
	p := newConfigPipeline(fs, &fakeReleaseSource{}, ConfigPipelineOptions{
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
	newConfigPipeline(fs, &fakeReleaseSource{}, ConfigPipelineOptions{Warn: func(msg string) { warned = msg }})

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
	newConfigPipeline(fs, &fakeReleaseSource{}, ConfigPipelineOptions{})

	if len(fs.renamed) != 0 {
		t.Errorf("no rename should happen when override already exists, got: %v", fs.renamed)
	}
}

func TestPipelineMigrationNoopWhenNothingExists(t *testing.T) {
	fs := &fakeFileSystem{}
	newConfigPipeline(fs, &fakeReleaseSource{}, ConfigPipelineOptions{})
	if len(fs.renamed) != 0 {
		t.Errorf("no rename should happen when neither file exists, got: %v", fs.renamed)
	}
}

func TestSetRemoteSubscriptionSelectsRemoteSourceAndClearsLocalState(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{subscriptionDataFile: true},
		written:    map[string][]byte{subscriptionDataFile: []byte("local: true\n")},
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

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
	source := &fakeReleaseSource{}
	m := NewConfigManager(fs, source, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}
	if source.downloadCalled {
		t.Fatal("local source should not download the stale remote URL")
	}
}

func TestInvalidSubscriptionSourceIsRejected(t *testing.T) {
	fs := &fakeFileSystem{
		written: map[string][]byte{subscriptionSourceFile: []byte("unknown\n")},
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

	_, err := m.PreviewConfig(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid subscription source") {
		t.Fatalf("PreviewConfig error = %v, want invalid source error", err)
	}
}

func TestSetSubscriptionSourceRejectsEmptySource(t *testing.T) {
	m := NewConfigManager(&fakeFileSystem{}, &fakeReleaseSource{}, &passValidator{}, nil)

	if err := m.SetSubscriptionSource(context.Background(), "  \n"); err == nil {
		t.Fatal("SetSubscriptionSource should reject an empty source")
	}
}

func TestUpdateConfigRejectsUnconfiguredSource(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true},
		written:    map[string][]byte{OverrideFilePath: []byte("mode: rule\n")},
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil || !strings.Contains(err.Error(), "subscription source is not configured") {
		t.Fatalf("UpdateConfig error = %v, want unconfigured source error", err)
	}
	requireConfigApplyFailure(t, m)
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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil || !strings.Contains(err.Error(), "URL is empty") {
		t.Fatalf("UpdateConfig error = %v, want empty URL error", err)
	}
}
