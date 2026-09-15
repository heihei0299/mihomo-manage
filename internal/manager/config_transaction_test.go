package manager

import (
	"context"
	"errors"
	"strings"
	"testing"
)

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

func TestUpdateConfigStagingDirectoryFailureRecordsApplyStatus(t *testing.T) {
	fs := localApplyTestFileSystem()
	fs.mkdirErrFunc = func(path string) error {
		if strings.Contains(path, ".mihomo-config-staging-") {
			return errors.New("staging directory unavailable")
		}
		return nil
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report staging directory failure")
	}
	requireConfigApplyFailure(t, m)
}

func TestUpdateConfigStagedWriteFailureRecordsApplyStatus(t *testing.T) {
	fs := localApplyTestFileSystem()
	fs.writeErrFunc = func(path string) error {
		if strings.Contains(path, ".mihomo-config-staging-") {
			return errors.New("staged config is not writable")
		}
		return nil
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report staged write failure")
	}
	requireConfigApplyFailure(t, m)
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
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report backup failure")
	}
	requireConfigApplyFailure(t, m)
}

func TestUpdateConfigRenameFailureRecordsApplyStatus(t *testing.T) {
	fs := localApplyTestFileSystem()
	fs.renameErrByPath = map[string]error{configYAML: errors.New("rename failed")}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report rename failure")
	}
	requireConfigApplyFailure(t, m)
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
	validator := &recordingValidator{}
	m := NewConfigManager(fs, &fakeReleaseSource{}, validator, nil)

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
