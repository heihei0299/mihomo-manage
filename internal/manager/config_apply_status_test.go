package manager

import (
	"context"
	"errors"
	"strings"
	"testing"
)

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

func requireConfigApplyFailure(t *testing.T, m ConfigManager) {
	t.Helper()
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
	m := NewConfigManager(fs, &fakeReleaseSource{downloadErr: errors.New("download failed")}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report download failure")
	}
	if _, ok := fs.written[tempPath]; ok {
		t.Fatal("download temp file should be removed after failure")
	}
	requireConfigApplyFailure(t, m)
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
	m := NewConfigManager(fs, &fakeReleaseSource{downloadErr: errors.New("download failed")}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report download failure")
	}
	requireConfigApplyFailure(t, m)
}

type failingStagingCleanupFileSystem struct {
	*fakeFileSystem
	err error
}

func (fs *failingStagingCleanupFileSystem) Remove(path string) error {
	if strings.Contains(path, ".mihomo-config-staging-") {
		return fs.err
	}
	return fs.fakeFileSystem.Remove(path)
}

func TestUpdateConfigStagingCleanupFailureRecordsApplyStatus(t *testing.T) {
	fs := &failingStagingCleanupFileSystem{
		fakeFileSystem: localApplyTestFileSystem(),
		err:            errors.New("staging cleanup failed"),
	}
	reloaded := false
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, func(context.Context) error {
		reloaded = true
		return nil
	})

	err := m.UpdateConfig(context.Background())
	if err == nil || !strings.Contains(err.Error(), "staging cleanup failed") {
		t.Fatalf("UpdateConfig error = %v, want staging cleanup failure", err)
	}
	if !reloaded {
		t.Fatal("UpdateConfig should reload before reporting cleanup failure")
	}
	status, statusErr := m.LastConfigApply(context.Background())
	if statusErr != nil {
		t.Fatalf("LastConfigApply failed: %v", statusErr)
	}
	if status.State != ConfigApplied || !strings.Contains(status.ErrorSummary, "staging cleanup failed") {
		t.Fatalf("status = %+v, want applied with cleanup error", status)
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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}
	status, err := m.LastConfigApply(context.Background())
	if err != nil {
		t.Fatalf("LastConfigApply failed: %v", err)
	}
	if status.State != ConfigApplied || status.ConfigHash == "" || status.AttemptedAt.IsZero() || status.ErrorSummary != "" {
		t.Fatalf("status = %+v, want applied status with timestamp, hash, and no error", status)
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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &failValidator{err: errors.New("invalid staged config")}, nil)

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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, func(context.Context) error {
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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

	status, err := m.LastConfigApply(context.Background())
	if err != nil {
		t.Fatalf("LastConfigApply failed: %v", err)
	}
	if status.State != ConfigUnknown || status.ErrorSummary == "" {
		t.Fatalf("status = %+v, want unknown with diagnostic", status)
	}
}

// passValidator is a ConfigValidator that always passes
