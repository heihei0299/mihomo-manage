package manager

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"runtime"
)

func serviceUnitPath() string {
	if runtime.GOOS == "darwin" {
		return "/Library/LaunchAgents/mihomo.plist"
	}
	return "/etc/systemd/system/mihomo.service"
}

func serviceUnitContent() []byte {
	if runtime.GOOS == "darwin" {
		return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>mihomo</string>
  <key>ProgramArguments</key>
  <array>
    <string>/opt/mihomo/bin/mihomo</string>
    <string>-d</string>
    <string>/opt/mihomo/etc</string>
  </array>
  <key>KeepAlive</key>
  <true/>
  <key>RunAtLoad</key>
  <true/>
</dict>
</plist>
`)
	}
	return []byte(`[Unit]
Description=mihomo (Clash Meta) proxy
After=network.target

[Service]
Type=simple
ExecStart=/opt/mihomo/bin/mihomo -d /opt/mihomo/etc
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`)
}

func releaseURL(goos, goarch, version string) string {
	return fmt.Sprintf("https://github.com/MetaCubeX/mihomo/releases/download/%s/mihomo-%s-%s-%s.gz", version, goos, goarch, version)
}

func (m *manager) resolveVersion(ctx context.Context, version string) string {
	if version != "latest" {
		return version
	}
	tag, err := m.sys.LatestVersion(ctx, "MetaCubeX", "mihomo")
	if err != nil || tag == "" {
		return version
	}
	return tag
}

func (m *manager) downloadAndDecompress(ctx context.Context, version string, onProgress ProgressCallback) (string, error) {
	version = m.resolveVersion(ctx, version)
	tempPath := fmt.Sprintf("%s.tmp.%s", binaryPath, version)
	gzPath := tempPath + ".gz"

	onProgress(ProgressEvent{Phase: PhaseFetch, Message: fmt.Sprintf("Downloading mihomo %s", version)})
	if err := m.sys.Download(ctx, releaseURL(runtime.GOOS, runtime.GOARCH, version), gzPath); err != nil {
		onProgress(ProgressEvent{Phase: PhaseFetch, Message: "Download failed", Error: err})
		m.sys.Remove(gzPath)
		return "", fmt.Errorf("download failed: %w", err)
	}
	onProgress(ProgressEvent{Phase: PhaseFetch, Message: "Decompressing"})
	if err := m.decompressGzip(gzPath, tempPath); err != nil {
		m.sys.Remove(gzPath)
		return "", fmt.Errorf("decompress failed: %w", err)
	}
	m.sys.Remove(gzPath)
	onProgress(ProgressEvent{Phase: PhaseFetch, Message: "Download complete"})
	return tempPath, nil
}

func (m *manager) decompressGzip(src, dest string) error {
	data, err := m.sys.ReadFile(src)
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
	return m.sys.WriteFile(dest, decompressed, filePermUserRWX)
}

func (m *manager) rollbackInstall(ctx context.Context, phase string, err error) error {
	m.svcMgr.Stop(serviceName)
	m.svcMgr.Unregister(serviceName)
	m.sys.Remove(binaryPath)
	m.sys.Remove(configDir)
	return fmt.Errorf("install failed at %s: %w", phase, err)
}

func (m *manager) Install(ctx context.Context, version string, onProgress ProgressCallback) error {
	tempPath, err := m.downloadAndDecompress(ctx, version, onProgress)
	if err != nil {
		return err
	}

	onProgress(ProgressEvent{Phase: PhaseDeploy, Message: "Deploying binary"})
	if err := m.sys.Rename(tempPath, binaryPath); err != nil {
		m.sys.Remove(tempPath)
		return m.rollbackInstall(ctx, "deploy rename", err)
	}
	onProgress(ProgressEvent{Phase: PhaseDeploy, Message: "Binary deployed"})

	onProgress(ProgressEvent{Phase: PhaseBootstrap, Message: "Creating config directory"})
	if err := m.sys.MkdirAll(configDir, filePermUserRWX); err != nil {
		return m.rollbackInstall(ctx, "bootstrap mkdir", err)
	}
	if err := m.sys.WriteFile(ConfigTemplatePath, defaultTemplate, filePermUserRW); err != nil {
		return m.rollbackInstall(ctx, "bootstrap template", err)
	}
	if err := m.sys.WriteFile(configYAML, defaultConfig, filePermUserRW); err != nil {
		return m.rollbackInstall(ctx, "bootstrap config", err)
	}
	svcPath := serviceUnitPath()
	svcContent := serviceUnitContent()
	if err := m.sys.WriteFile(svcPath, svcContent, filePermUserRW); err != nil {
		return m.rollbackInstall(ctx, "bootstrap service unit", err)
	}
	onProgress(ProgressEvent{Phase: PhaseBootstrap, Message: "Config files created"})

	onProgress(ProgressEvent{Phase: PhaseRegister, Message: "Registering system service"})
	if err := m.svcMgr.Register(serviceName, svcPath); err != nil {
		return m.rollbackInstall(ctx, "service register", err)
	}
	onProgress(ProgressEvent{Phase: PhaseRegister, Message: "Service registered"})

	onProgress(ProgressEvent{Phase: PhaseStart, Message: "Starting mihomo"})
	if err := m.svcMgr.Start(serviceName); err != nil {
		return m.rollbackInstall(ctx, "service start", err)
	}
	onProgress(ProgressEvent{Phase: PhaseStart, Message: "mihomo is running"})

	return nil
}

func (m *manager) Uninstall(ctx context.Context, keepBackup bool, onProgress ProgressCallback) error {
	if !m.sys.FileExists(binaryPath) {
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
		m.sys.Rename("/opt/mihomo", backupPath)
		onProgress(ProgressEvent{Phase: PhaseUninstallCleanup, Message: "Files backed up to " + backupPath})
	} else {
		m.sys.Remove(binaryPath + ".bak.")
		m.sys.Remove(configDir)
		m.sys.Remove("/opt/mihomo")
		m.sys.Remove("/opt/mihomo-manager")
		onProgress(ProgressEvent{Phase: PhaseUninstallCleanup, Message: "Files removed"})
	}

	return nil
}

func (m *manager) Upgrade(ctx context.Context, version string, onProgress ProgressCallback) error {
	version = m.resolveVersion(ctx, version)
	if !m.sys.FileExists(binaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}

	tempPath, err := m.downloadAndDecompress(ctx, version, onProgress)
	if err != nil {
		return err
	}

	onProgress(ProgressEvent{Phase: PhaseUpgradeStop, Message: "Stopping mihomo"})
	if running, _ := m.svcMgr.IsRunning(serviceName); running {
		if err := m.svcMgr.Stop(serviceName); err != nil {
			m.sys.Remove(tempPath)
			return fmt.Errorf("stop failed: %w", err)
		}
	}
	onProgress(ProgressEvent{Phase: PhaseUpgradeStop, Message: "Stopped"})

	onProgress(ProgressEvent{Phase: PhaseUpgradeReplace, Message: "Backing up old binary"})
	backupDir := "/opt/mihomo-manager/backups"
	m.sys.MkdirAll(backupDir, filePermUserRWX)
	backupPath := backupDir + "/mihomo.bak"
	m.sys.Rename(binaryPath, backupPath)

	onProgress(ProgressEvent{Phase: PhaseUpgradeReplace, Message: "Replacing binary"})
	if err := m.sys.Chmod(tempPath, filePermUserRWX); err != nil {
		m.restoreBinary(backupPath, tempPath)
		return fmt.Errorf("chmod failed: %w", err)
	}
	if err := m.sys.Rename(tempPath, binaryPath); err != nil {
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

func (m *manager) restoreBinary(backupPath, tempPath string) error {
	m.sys.Remove(binaryPath)
	m.sys.Rename(backupPath, binaryPath)
	m.sys.Remove(tempPath)
	return m.svcMgr.Start(serviceName)
}
