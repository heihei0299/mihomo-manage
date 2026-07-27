package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/anomalyco/mihomo-manager/internal/config"
	"github.com/anomalyco/mihomo-manager/internal/domain"
	"github.com/anomalyco/mihomo-manager/internal/infra"
)

type lifecycleManager struct {
	fs     infra.FileSystem
	cmd    infra.CommandRunner
	gh     infra.ReleaseRepo
	svcMgr domain.ServiceManager
}

func NewLifecycle(fs infra.FileSystem, cmd infra.CommandRunner, gh infra.ReleaseRepo, svcMgr domain.ServiceManager) domain.LifecycleManager {
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

func (m *lifecycleManager) downloadAndDecompress(ctx context.Context, version string, onProgress domain.ProgressCallback) (string, error) {
	version = m.resolveVersion(ctx, version)
	tempPath := fmt.Sprintf("%s.tmp.%s", config.BinaryPath, version)
	gzPath := tempPath + ".gz"

	onProgress(domain.ProgressEvent{Phase: domain.PhaseFetch, Message: fmt.Sprintf("Downloading mihomo %s", version)})
	if err := m.gh.Download(ctx, releaseURL(runtime.GOOS, runtime.GOARCH, version), gzPath); err != nil {
		onProgress(domain.ProgressEvent{Phase: domain.PhaseFetch, Message: "Download failed", Error: err})
		m.fs.Remove(gzPath)
		return "", fmt.Errorf("download failed: %w", err)
	}
	onProgress(domain.ProgressEvent{Phase: domain.PhaseFetch, Message: "Decompressing"})
	if err := m.decompressGzip(gzPath, tempPath); err != nil {
		m.fs.Remove(gzPath)
		return "", fmt.Errorf("decompress failed: %w", err)
	}
	m.fs.Remove(gzPath)
	onProgress(domain.ProgressEvent{Phase: domain.PhaseFetch, Message: "Download complete"})
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
	return m.fs.WriteFile(dest, decompressed, config.FilePermUserRWX)
}

func (m *lifecycleManager) rollbackInstall(ctx context.Context, phase string, err error) error {
	m.svcMgr.Stop(config.ServiceName)
	m.svcMgr.Unregister(config.ServiceName)
	m.fs.Remove(config.BinaryPath)
	m.fs.Remove(config.ConfigDir)
	return fmt.Errorf("install failed at %s: %w", phase, err)
}

func (m *lifecycleManager) Install(ctx context.Context, version string, autoStart bool, onProgress domain.ProgressCallback) error {
	tempPath, err := m.downloadAndDecompress(ctx, version, onProgress)
	if err != nil {
		return err
	}
	return m.installBinary(ctx, tempPath, autoStart, onProgress)
}

func (m *lifecycleManager) InstallFromLocal(ctx context.Context, localPath string, autoStart bool, onProgress domain.ProgressCallback) error {
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
		tempPath := config.BinaryPath + ".tmp.local"
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

func (m *lifecycleManager) installBinary(ctx context.Context, binarySrc string, autoStart bool, onProgress domain.ProgressCallback) error {
	onProgress(domain.ProgressEvent{Phase: domain.PhaseDeploy, Message: "Deploying binary"})
	if err := m.fs.Rename(binarySrc, config.BinaryPath); err != nil {
		m.fs.Remove(binarySrc)
		return m.rollbackInstall(ctx, "deploy rename", err)
	}
	onProgress(domain.ProgressEvent{Phase: domain.PhaseDeploy, Message: "Binary deployed"})

	onProgress(domain.ProgressEvent{Phase: domain.PhaseBootstrap, Message: "Creating directories"})
	if err := m.fs.MkdirAll(config.ConfigDir, config.FilePermUserRWX); err != nil {
		return m.rollbackInstall(ctx, "bootstrap mkdir config", err)
	}
	if err := m.fs.MkdirAll(config.StateDir, config.FilePermUserRWX); err != nil {
		return m.rollbackInstall(ctx, "bootstrap mkdir state", err)
	}
	if err := m.fs.WriteFile(config.ConfigTemplatePath, config.DefaultTemplate, config.FilePermUserRW); err != nil {
		return m.rollbackInstall(ctx, "bootstrap template", err)
	}
	if err := m.fs.WriteFile(config.ConfigYAML, config.DefaultConfig, config.FilePermUserRW); err != nil {
		return m.rollbackInstall(ctx, "bootstrap config", err)
	}
	svcPath := serviceUnitPath()
	svcContent := serviceUnitContent(autoStart)
	if err := m.fs.WriteFile(svcPath, svcContent, config.FilePermUserRW); err != nil {
		return m.rollbackInstall(ctx, "bootstrap service unit", err)
	}
	onProgress(domain.ProgressEvent{Phase: domain.PhaseBootstrap, Message: "Config files created"})

	onProgress(domain.ProgressEvent{Phase: domain.PhaseRegister, Message: "Registering system service"})
	if err := m.svcMgr.Register(config.ServiceName, svcPath); err != nil {
		return m.rollbackInstall(ctx, "service register", err)
	}
	onProgress(domain.ProgressEvent{Phase: domain.PhaseRegister, Message: "Service registered"})

	if autoStart {
		onProgress(domain.ProgressEvent{Phase: domain.PhaseEnableAutoStart, Message: "Enabling auto-start"})
		if err := m.svcMgr.EnableAutoStart(config.ServiceName, svcPath); err != nil {
			return m.rollbackInstall(ctx, "enable auto-start", err)
		}
		onProgress(domain.ProgressEvent{Phase: domain.PhaseEnableAutoStart, Message: "Auto-start enabled"})
	}

	onProgress(domain.ProgressEvent{Phase: domain.PhaseStart, Message: "Starting mihomo"})
	if err := m.svcMgr.Start(config.ServiceName); err != nil {
		return m.rollbackInstall(ctx, "service start", err)
	}
	onProgress(domain.ProgressEvent{Phase: domain.PhaseStart, Message: "mihomo is running"})

	return nil
}

func (m *lifecycleManager) Uninstall(ctx context.Context, keepBackup bool, onProgress domain.ProgressCallback) error {
	if !m.fs.FileExists(config.BinaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}

	onProgress(domain.ProgressEvent{Phase: domain.PhaseUninstallStop, Message: "Stopping mihomo"})
	if running, _ := m.svcMgr.IsRunning(config.ServiceName); running {
		m.svcMgr.Stop(config.ServiceName)
	}
	onProgress(domain.ProgressEvent{Phase: domain.PhaseUninstallStop, Message: "Stopped"})

	onProgress(domain.ProgressEvent{Phase: domain.PhaseUninstallDeregister, Message: "Removing service"})
	m.svcMgr.Unregister(config.ServiceName)
	onProgress(domain.ProgressEvent{Phase: domain.PhaseUninstallDeregister, Message: "Service removed"})

	onProgress(domain.ProgressEvent{Phase: domain.PhaseUninstallCleanup, Message: "Cleaning up files"})
	if keepBackup {
		backupPath := "/opt/mihomo.bak." + domain.Timestamp()
		m.fs.Rename("/opt/mihomo", backupPath)
		onProgress(domain.ProgressEvent{Phase: domain.PhaseUninstallCleanup, Message: "Files backed up to " + backupPath})
	} else {
		m.fs.Remove(config.BinaryPath + ".bak.")
		m.fs.Remove(config.ConfigDir)
		m.fs.Remove("/opt/mihomo")
		m.fs.Remove("/opt/mihomo-manager")
		onProgress(domain.ProgressEvent{Phase: domain.PhaseUninstallCleanup, Message: "Files removed"})
	}

	return nil
}

func (m *lifecycleManager) Upgrade(ctx context.Context, version string, onProgress domain.ProgressCallback) error {
	version = m.resolveVersion(ctx, version)
	if !m.fs.FileExists(config.BinaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}

	tempPath, err := m.downloadAndDecompress(ctx, version, onProgress)
	if err != nil {
		return err
	}

	onProgress(domain.ProgressEvent{Phase: domain.PhaseUpgradeStop, Message: "Stopping mihomo"})
	if running, _ := m.svcMgr.IsRunning(config.ServiceName); running {
		if err := m.svcMgr.Stop(config.ServiceName); err != nil {
			m.fs.Remove(tempPath)
			return fmt.Errorf("stop failed: %w", err)
		}
	}
	onProgress(domain.ProgressEvent{Phase: domain.PhaseUpgradeStop, Message: "Stopped"})

	onProgress(domain.ProgressEvent{Phase: domain.PhaseUpgradeReplace, Message: "Backing up old binary"})
	backupDir := "/opt/mihomo-manager/backups"
	m.fs.MkdirAll(backupDir, config.FilePermUserRWX)
	backupPath := backupDir + "/mihomo.bak"
	m.fs.Rename(config.BinaryPath, backupPath)

	onProgress(domain.ProgressEvent{Phase: domain.PhaseUpgradeReplace, Message: "Replacing binary"})
	if err := m.fs.Chmod(tempPath, config.FilePermUserRWX); err != nil {
		m.restoreBinary(backupPath, tempPath)
		return fmt.Errorf("chmod failed: %w", err)
	}
	if err := m.fs.Rename(tempPath, config.BinaryPath); err != nil {
		m.restoreBinary(backupPath, tempPath)
		return fmt.Errorf("rename failed: %w", err)
	}
	onProgress(domain.ProgressEvent{Phase: domain.PhaseUpgradeReplace, Message: "Binary replaced"})

	onProgress(domain.ProgressEvent{Phase: domain.PhaseUpgradeStart, Message: "Starting mihomo"})
	if err := m.svcMgr.Start(config.ServiceName); err != nil {
		if rbErr := m.restoreBinary(backupPath, ""); rbErr != nil {
			return fmt.Errorf("start failed, rollback also failed: %v (original: %w)", rbErr, err)
		}
		return fmt.Errorf("start failed, rolled back: %w", err)
	}
	onProgress(domain.ProgressEvent{Phase: domain.PhaseUpgradeStart, Message: "Running " + version})

	return nil
}

func (m *lifecycleManager) restoreBinary(backupPath, tempPath string) error {
	m.fs.Remove(config.BinaryPath)
	m.fs.Rename(backupPath, config.BinaryPath)
	m.fs.Remove(tempPath)
	return m.svcMgr.Start(config.ServiceName)
}

func (m *lifecycleManager) ListVersions(ctx context.Context) ([]domain.VersionInfo, error) {
	return m.gh.ListVersions(ctx, "MetaCubeX", "mihomo", 5)
}
