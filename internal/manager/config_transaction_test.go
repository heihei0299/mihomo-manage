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

type failingRenameFileSystem struct {
	*fakeFileSystem
	err error
}

func (fs *failingRenameFileSystem) Rename(oldPath, newPath string) error {
	if newPath == configYAML {
		return fs.err
	}
	return fs.fakeFileSystem.Rename(oldPath, newPath)
}

type failingMkdirFileSystem struct {
	*fakeFileSystem
	err error
}

func (fs *failingMkdirFileSystem) MkdirAll(path string, perm uint32) error {
	if strings.Contains(path, ".mihomo-config-staging-") {
		return fs.err
	}
	return fs.fakeFileSystem.MkdirAll(path, perm)
}

type failingStagedWriteFileSystem struct {
	*fakeFileSystem
	err error
}

func (fs *failingStagedWriteFileSystem) WriteFile(path string, data []byte, perm uint32) error {
	if strings.Contains(path, ".mihomo-config-staging-") {
		return fs.err
	}
	return fs.fakeFileSystem.WriteFile(path, data, perm)
}

type failingBackupWriteFileSystem struct {
	*fakeFileSystem
	err error
}

func (fs *failingBackupWriteFileSystem) WriteFile(path string, data []byte, perm uint32) error {
	if strings.HasPrefix(path, configYAML+".bak.") {
		return fs.err
	}
	return fs.fakeFileSystem.WriteFile(path, data, perm)
}

func TestUpdateConfigStagingDirectoryFailureRecordsApplyStatus(t *testing.T) {
	fs := &failingMkdirFileSystem{
		fakeFileSystem: localApplyTestFileSystem(),
		err:            errors.New("staging directory unavailable"),
	}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report staging directory failure")
	}
	requireConfigApplyFailure(t, m)
}

func TestUpdateConfigStagedWriteFailureRecordsApplyStatus(t *testing.T) {
	fs := &failingStagedWriteFileSystem{
		fakeFileSystem: localApplyTestFileSystem(),
		err:            errors.New("staged config is not writable"),
	}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report staged write failure")
	}
	requireConfigApplyFailure(t, m)
}

func TestUpdateConfigBackupFailureRecordsApplyStatus(t *testing.T) {
	base := localApplyTestFileSystem()
	base.fileExists[configYAML] = true
	base.written[configYAML] = []byte("old\n")
	fs := &failingBackupWriteFileSystem{
		fakeFileSystem: base,
		err:            errors.New("backup is not writable"),
	}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report backup failure")
	}
	requireConfigApplyFailure(t, m)
}

func TestUpdateConfigRenameFailureRecordsApplyStatus(t *testing.T) {
	fs := &failingRenameFileSystem{
		fakeFileSystem: localApplyTestFileSystem(),
		err:            errors.New("rename failed"),
	}
	m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

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
	m := newTestConfigManager(fs, &fakeReleaseSource{}, validator, noopReload)

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
