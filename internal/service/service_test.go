package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"os"

	"github.com/anomalyco/mihomo-manager/internal/domain"
)

// fakeFileSystem implements infra.FileSystem for testing.
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

// linkStorage makes gh share the same written map as fs, so
// Download writes are visible to ReadFile.
func linkStorage(fs *fakeFileSystem, gh *fakeGitHubReleases) {
	if fs.written == nil {
		fs.written = make(map[string][]byte)
	}
	gh.written = fs.written
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
