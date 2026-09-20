package manager

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type failOnceWriteFileSystem struct {
	*fakeFileSystem
	path string
	err  error
}

func (fs *failOnceWriteFileSystem) WriteFile(path string, data []byte, perm uint32) error {
	if path == fs.path && fs.err != nil {
		err := fs.err
		fs.err = nil
		return err
	}
	return fs.fakeFileSystem.WriteFile(path, data, perm)
}

type readFileErrorFileSystem struct {
	*fakeFileSystem
	path string
	err  error
}

func (fs *readFileErrorFileSystem) ReadFile(path string) ([]byte, error) {
	if path == fs.path {
		return nil, fs.err
	}
	return fs.fakeFileSystem.ReadFile(path)
}

func TestUpdateConfigReadURLError(t *testing.T) {
	fs := &readFileErrorFileSystem{
		fakeFileSystem: &fakeFileSystem{
			fileExists: map[string]bool{
				OverrideFilePath:    true,
				subscriptionURLFile: true,
			},
			written: map[string][]byte{
				OverrideFilePath: []byte(`test: {{subscription}}`),
			},
		},
		path: subscriptionURLFile,
		err:  testError{"permission denied"},
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &configValidator{}, noopReload)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Error("expected error when ReadFile fails on subscriptionURLFile")
	}
}

func TestPreviewConfigSubscriptionReadError(t *testing.T) {
	fs := &readFileErrorFileSystem{
		fakeFileSystem: &fakeFileSystem{
			written: map[string][]byte{
				OverrideFilePath: []byte(`test: {{subscription}}`),
			},
		},
		path: subscriptionDataFile,
		err:  testError{"permission denied"},
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &configValidator{}, noopReload)

	if _, err := m.PreviewConfig(context.Background()); err == nil {
		t.Error("expected error when ReadFile fails on subscriptionDataFile with non-ErrNotExist error")
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
	p := newConfigPipeline(fs, &fakeReleaseSource{}, configPipelineOptions{
		Warn: func(msg string) { warned = msg },
	})

	_, err := p.PreviewConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(warned, "old placeholder") {
		t.Errorf("expected deprecation warning, got: %q", warned)
	}
}

func TestConfigManagerConstructionHasNoSideEffects(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{legacyTemplatePath: true},
		written:    map[string][]byte{legacyTemplatePath: []byte("port: 8888\n")},
	}
	lock := &fakeConfigUpdateLock{}
	warned := false

	NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload,
		WithConfigUpdateLock(lock),
		WithConfigWarning(func(string) { warned = true }),
	)

	if fs.accessed || lock.acquired || warned {
		t.Fatalf("constructor performed side effects: filesystem=%v lock=%v warning=%v", fs.accessed, lock.acquired, warned)
	}
}

func TestConfigManagerPreparationReturnsFileSystemErrors(t *testing.T) {
	wantErr := errors.New("stat failed")
	m := NewConfigManager(&fakeFileSystem{fileExistsErr: wantErr}, &fakeReleaseSource{}, &passValidator{}, noopReload)

	if _, err := m.PreviewConfig(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("PreviewConfig error = %v, want %v", err, wantErr)
	}
}

func TestPipelineMigratesLegacyTemplate(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{legacyTemplatePath: true},
		written:    map[string][]byte{legacyTemplatePath: []byte("port: 8888\n")},
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)
	if _, err := m.PreviewConfig(context.Background()); err != nil {
		t.Fatalf("PreviewConfig failed: %v", err)
	}

	if exists, _ := fs.FileExists(OverrideFilePath); !exists {
		t.Errorf("legacy template should be migrated to %s", OverrideFilePath)
	}
	if exists, _ := fs.FileExists(legacyTemplatePath); exists {
		t.Errorf("legacy template should no longer exist after migration")
	}
	data, err := fs.ReadFile(OverrideFilePath)
	if err != nil || string(data) != "port: 8888\n" {
		t.Errorf("migrated override should carry legacy content, got %q err %v", data, err)
	}
}

func TestPipelineMigrationSkipsWhenOverrideExists(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{legacyTemplatePath: true, OverrideFilePath: true},
		written:    map[string][]byte{OverrideFilePath: []byte("port: 9999\n")},
	}
	p := newConfigPipeline(fs, &fakeReleaseSource{}, configPipelineOptions{})
	if _, err := p.PreviewConfig(context.Background()); err != nil {
		t.Fatalf("PreviewConfig failed: %v", err)
	}

	if len(fs.renamed) != 0 {
		t.Errorf("no rename should happen when override already exists, got: %v", fs.renamed)
	}
}

func TestPipelineMigrationNoopWhenNothingExists(t *testing.T) {
	fs := &fakeFileSystem{}
	p := newConfigPipeline(fs, &fakeReleaseSource{}, configPipelineOptions{})
	if _, err := p.PreviewConfig(context.Background()); err != nil {
		t.Fatalf("PreviewConfig failed: %v", err)
	}
	if len(fs.renamed) != 0 {
		t.Errorf("no rename should happen when neither file exists, got: %v", fs.renamed)
	}
}

func TestSetRemoteSubscriptionSelectsRemoteSourceAndClearsLocalState(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{subscriptionDataFile: true},
		written:    map[string][]byte{subscriptionDataFile: []byte("local: true\n")},
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

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
	fs := &failOnceWriteFileSystem{
		fakeFileSystem: &fakeFileSystem{
			fileExists: map[string]bool{
				subscriptionSourceFile: true,
				subscriptionDataFile:   true,
			},
			written: map[string][]byte{
				subscriptionSourceFile: []byte("local\n"),
				subscriptionDataFile:   []byte("mode: rule\n"),
			},
		},
		path: subscriptionSourceFile,
		err:  errors.New("marker write failed"),
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

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
	m := NewConfigManager(fs, source, &passValidator{}, noopReload)

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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	_, err := m.PreviewConfig(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid subscription source") {
		t.Fatalf("PreviewConfig error = %v, want invalid source error", err)
	}
}

func TestSetSubscriptionSourceRejectsEmptySource(t *testing.T) {
	m := NewConfigManager(&fakeFileSystem{}, &fakeReleaseSource{}, &passValidator{}, noopReload)

	if err := m.SetSubscriptionSource(context.Background(), "  \n"); err == nil {
		t.Fatal("SetSubscriptionSource should reject an empty source")
	}
}

func TestUpdateConfigRejectsUnconfiguredSource(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true},
		written:    map[string][]byte{OverrideFilePath: []byte("mode: rule\n")},
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	if err := m.UpdateConfig(context.Background()); err == nil || !strings.Contains(err.Error(), "URL is empty") {
		t.Fatalf("UpdateConfig error = %v, want empty URL error", err)
	}
}
