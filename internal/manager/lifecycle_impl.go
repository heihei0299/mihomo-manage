package manager

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"
)

type lifecycleManager struct {
	fs     FileSystem
	cmd    CommandRunner
	gh     GitHubReleases
	svcMgr ServiceManager
}

func NewLifecycleManager(fs FileSystem, cmd CommandRunner, gh GitHubReleases, svcMgr ServiceManager) LifecycleManager {
	return &lifecycleManager{fs: fs, cmd: cmd, gh: gh, svcMgr: svcMgr}
}

func (m *lifecycleManager) resolveVersion(ctx context.Context, version string) string {
	if version != "latest" {
		return version
	}
	tag, err := m.gh.LatestVersion(ctx, "MetaCubeX", "mihomo")
	if err != nil || tag == "" {
		return version
	}
	return tag
}

func (m *lifecycleManager) downloadAndDecompress(ctx context.Context, version string, onProgress ProgressCallback) (string, error) {
	version = m.resolveVersion(ctx, version)
	tempPath := fmt.Sprintf("%s.tmp.%s", binaryPath, version)
	gzPath := tempPath + ".gz"

	onProgress(ProgressEvent{Phase: PhaseFetch, Message: fmt.Sprintf("Downloading mihomo %s", version)})
	if err := m.gh.Download(ctx, releaseURL(runtime.GOOS, runtime.GOARCH, version), gzPath); err != nil {
		onProgress(ProgressEvent{Phase: PhaseFetch, Message: "Download failed", Error: err})
		m.fs.Remove(gzPath)
		return "", fmt.Errorf("download failed: %w", err)
	}
	onProgress(ProgressEvent{Phase: PhaseFetch, Message: "Decompressing"})
	if err := m.decompressGzip(gzPath, tempPath); err != nil {
		m.fs.Remove(gzPath)
		return "", fmt.Errorf("decompress failed: %w", err)
	}
	m.fs.Remove(gzPath)
	onProgress(ProgressEvent{Phase: PhaseFetch, Message: "Download complete"})
	return tempPath, nil
}

func (m *lifecycleManager) decompressGzip(src, dest string) error {
	data, err := m.fs.ReadFile(src)
	if err != nil {
		return err
	}
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decompress: %w", err)
	}
	defer gr.Close()
	decompressed, err := io.ReadAll(gr)
	if err != nil {
		return fmt.Errorf("decompress read: %w", err)
	}
	gr.Close()
	return m.fs.WriteFile(dest, decompressed, filePermUserRWX)
}

func (m *lifecycleManager) rollbackInstall(ctx context.Context, phase string, err error) error {
	m.svcMgr.Stop(serviceName)
	m.svcMgr.Unregister(serviceName)
	m.fs.Remove(binaryPath)
	m.fs.Remove(configDir)
	return fmt.Errorf("install failed at %s: %w", phase, err)
}

func (m *lifecycleManager) Install(ctx context.Context, version string, autoStart bool, onProgress ProgressCallback) error {
	tempPath, err := m.downloadAndDecompress(ctx, version, onProgress)
	if err != nil {
		return err
	}
	return m.installBinary(ctx, tempPath, autoStart, onProgress)
}

func (m *lifecycleManager) InstallFromLocal(ctx context.Context, localPath string, autoStart bool, onProgress ProgressCallback) error {
	tempPath, err := m.resolveLocalBinary(localPath)
	if err != nil {
		return fmt.Errorf("local binary: %w", err)
	}
	defer func() {
		if tempPath != localPath {
			m.fs.Remove(tempPath)
		}
	}()
	return m.installBinary(ctx, tempPath, autoStart, onProgress)
}

func (m *lifecycleManager) resolveLocalBinary(localPath string) (string, error) {
	if strings.HasSuffix(localPath, ".gz") {
		tempPath := binaryPath + ".tmp.local"
		if err := m.decompressGzip(localPath, tempPath); err != nil {
			return "", err
		}
		return tempPath, nil
	}
	if !m.fs.FileExists(localPath) {
		return "", fmt.Errorf("file not found: %s", localPath)
	}
	return localPath, nil
}

func (m *lifecycleManager) installBinary(ctx context.Context, binarySrc string, autoStart bool, onProgress ProgressCallback) error {
	onProgress(ProgressEvent{Phase: PhaseDeploy, Message: "Deploying binary"})
	if err := m.fs.Rename(binarySrc, binaryPath); err != nil {
		m.fs.Remove(binarySrc)
		return m.rollbackInstall(ctx, "deploy rename", err)
	}
	onProgress(ProgressEvent{Phase: PhaseDeploy, Message: "Binary deployed"})

	onProgress(ProgressEvent{Phase: PhaseBootstrap, Message: "Creating directories"})
	if err := m.fs.MkdirAll(configDir, filePermUserRWX); err != nil {
		return m.rollbackInstall(ctx, "bootstrap mkdir config", err)
	}
	if err := m.fs.MkdirAll(stateDir, filePermUserRWX); err != nil {
		return m.rollbackInstall(ctx, "bootstrap mkdir state", err)
	}
	if err := m.fs.WriteFile(ConfigTemplatePath, defaultTemplate, filePermUserRW); err != nil {
		return m.rollbackInstall(ctx, "bootstrap template", err)
	}
	if err := m.fs.WriteFile(configYAML, defaultConfig, filePermUserRW); err != nil {
		return m.rollbackInstall(ctx, "bootstrap config", err)
	}
	svcPath := serviceUnitPath()
	svcContent := serviceUnitContent(autoStart)
	if err := m.fs.WriteFile(svcPath, svcContent, filePermUserRW); err != nil {
		return m.rollbackInstall(ctx, "bootstrap service unit", err)
	}
	onProgress(ProgressEvent{Phase: PhaseBootstrap, Message: "Config files created"})

	onProgress(ProgressEvent{Phase: PhaseRegister, Message: "Registering system service"})
	if err := m.svcMgr.Register(serviceName, svcPath); err != nil {
		return m.rollbackInstall(ctx, "service register", err)
	}
	onProgress(ProgressEvent{Phase: PhaseRegister, Message: "Service registered"})

	if autoStart {
		onProgress(ProgressEvent{Phase: PhaseEnableAutoStart, Message: "Enabling auto-start"})
		if err := m.svcMgr.EnableAutoStart(serviceName, svcPath); err != nil {
			return m.rollbackInstall(ctx, "enable auto-start", err)
		}
		onProgress(ProgressEvent{Phase: PhaseEnableAutoStart, Message: "Auto-start enabled"})
	}

	onProgress(ProgressEvent{Phase: PhaseStart, Message: "Starting mihomo"})
	if err := m.svcMgr.Start(serviceName); err != nil {
		return m.rollbackInstall(ctx, "service start", err)
	}
	onProgress(ProgressEvent{Phase: PhaseStart, Message: "mihomo is running"})

	return nil
}

func (m *lifecycleManager) Uninstall(ctx context.Context, keepBackup bool, onProgress ProgressCallback) error {
	if !m.fs.FileExists(binaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}

	onProgress(ProgressEvent{Phase: PhaseUninstallStop, Message: "Stopping mihomo"})
	if running, _ := m.svcMgr.IsRunning(serviceName); running {
		m.svcMgr.Stop(serviceName)
	}
	onProgress(ProgressEvent{Phase: PhaseUninstallStop, Message: "Stopped"})

	onProgress(ProgressEvent{Phase: PhaseUninstallDeregister, Message: "Removing service"})
	m.svcMgr.Unregister(serviceName)
	onProgress(ProgressEvent{Phase: PhaseUninstallDeregister, Message: "Service removed"})

	onProgress(ProgressEvent{Phase: PhaseUninstallCleanup, Message: "Cleaning up files"})
	if keepBackup {
		backupPath := "/opt/mihomo.bak." + timestamp()
		m.fs.Rename("/opt/mihomo", backupPath)
		onProgress(ProgressEvent{Phase: PhaseUninstallCleanup, Message: "Files backed up to " + backupPath})
	} else {
		m.fs.Remove(binaryPath + ".bak.")
		m.fs.Remove(configDir)
		m.fs.Remove("/opt/mihomo")
		m.fs.Remove("/opt/mihomo-manager")
		onProgress(ProgressEvent{Phase: PhaseUninstallCleanup, Message: "Files removed"})
	}

	return nil
}

func (m *lifecycleManager) Upgrade(ctx context.Context, version string, onProgress ProgressCallback) error {
	version = m.resolveVersion(ctx, version)
	if !m.fs.FileExists(binaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}

	tempPath, err := m.downloadAndDecompress(ctx, version, onProgress)
	if err != nil {
		return err
	}

	onProgress(ProgressEvent{Phase: PhaseUpgradeStop, Message: "Stopping mihomo"})
	if running, _ := m.svcMgr.IsRunning(serviceName); running {
		if err := m.svcMgr.Stop(serviceName); err != nil {
			m.fs.Remove(tempPath)
			return fmt.Errorf("stop failed: %w", err)
		}
	}
	onProgress(ProgressEvent{Phase: PhaseUpgradeStop, Message: "Stopped"})

	onProgress(ProgressEvent{Phase: PhaseUpgradeReplace, Message: "Backing up old binary"})
	backupDir := "/opt/mihomo-manager/backups"
	m.fs.MkdirAll(backupDir, filePermUserRWX)
	backupPath := backupDir + "/mihomo.bak"
	m.fs.Rename(binaryPath, backupPath)

	onProgress(ProgressEvent{Phase: PhaseUpgradeReplace, Message: "Replacing binary"})
	if err := m.fs.Chmod(tempPath, filePermUserRWX); err != nil {
		m.restoreBinary(backupPath, tempPath)
		return fmt.Errorf("chmod failed: %w", err)
	}
	if err := m.fs.Rename(tempPath, binaryPath); err != nil {
		m.restoreBinary(backupPath, tempPath)
		return fmt.Errorf("rename failed: %w", err)
	}
	onProgress(ProgressEvent{Phase: PhaseUpgradeReplace, Message: "Binary replaced"})

	onProgress(ProgressEvent{Phase: PhaseUpgradeStart, Message: "Starting mihomo"})
	if err := m.svcMgr.Start(serviceName); err != nil {
		if rbErr := m.restoreBinary(backupPath, ""); rbErr != nil {
			return fmt.Errorf("start failed, rollback also failed: %v (original: %w)", rbErr, err)
		}
		return fmt.Errorf("start failed, rolled back: %w", err)
	}
	onProgress(ProgressEvent{Phase: PhaseUpgradeStart, Message: "Running " + version})

	return nil
}

func (m *lifecycleManager) restoreBinary(backupPath, tempPath string) error {
	m.fs.Remove(binaryPath)
	m.fs.Rename(backupPath, binaryPath)
	m.fs.Remove(tempPath)
	return m.svcMgr.Start(serviceName)
}

func (m *lifecycleManager) ListVersions(ctx context.Context) ([]VersionInfo, error) {
	return m.gh.ListVersions(ctx, "MetaCubeX", "mihomo", 5)
}
