package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	systemdScheduleService = "/etc/systemd/system/mihomo-manager-subscription-update.service"
	systemdScheduleTimer   = "/etc/systemd/system/mihomo-manager-subscription-update.timer"
	systemdScheduleName    = "mihomo-manager-subscription-update.timer"
	launchdSchedulePlist   = "/Library/LaunchDaemons/mihomo-manager-subscription-update.plist"
	launchdScheduleLabel   = "mihomo-manager-subscription-update"
)

type PlatformScheduler interface {
	Set(ctx context.Context, interval time.Duration, commandPath string) error
	Stop(ctx context.Context) error
	Status(ctx context.Context) (time.Duration, bool, error)
}

type linuxPlatformScheduler struct {
	fs  FileSystem
	cmd CommandRunner
}

func NewLinuxPlatformScheduler(fs FileSystem, cmd CommandRunner) PlatformScheduler {
	return &linuxPlatformScheduler{fs: fs, cmd: cmd}
}

func NewDarwinPlatformScheduler(fs FileSystem, cmd CommandRunner) PlatformScheduler {
	return &darwinPlatformScheduler{fs: fs, cmd: cmd}
}

func (s *linuxPlatformScheduler) Set(ctx context.Context, interval time.Duration, commandPath string) error {
	if interval < time.Hour {
		return fmt.Errorf("minimum interval is 1h, got %v", interval)
	}
	service := fmt.Sprintf("[Unit]\nDescription=mihomo subscription update\n\n[Service]\nType=oneshot\nExecStart=%s subscription update --quiet\n", commandPath)
	timer := fmt.Sprintf("[Unit]\nDescription=Scheduled mihomo subscription update\n\n[Timer]\nOnUnitActiveSec=%ds\nPersistent=false\nUnit=%s\n\n[Install]\nWantedBy=timers.target\n", int64(interval.Seconds()), strings.TrimSuffix(systemdScheduleName, ".timer")+".service")
	if err := s.fs.WriteFile(systemdScheduleService, []byte(service), filePermUserRW); err != nil {
		return fmt.Errorf("writing systemd schedule service: %w", err)
	}
	if err := s.fs.WriteFile(systemdScheduleTimer, []byte(timer), filePermUserRW); err != nil {
		return fmt.Errorf("writing systemd schedule timer: %w", err)
	}
	if _, err := s.cmd.RunCommand(ctx, "systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemd daemon-reload: %w", err)
	}
	if _, err := s.cmd.RunCommand(ctx, "systemctl", "enable", "--now", systemdScheduleName); err != nil {
		return fmt.Errorf("enabling systemd schedule: %w", err)
	}
	return nil
}

func (s *linuxPlatformScheduler) Stop(ctx context.Context) error {
	active, exists, enabled, err := s.timerState(ctx)
	if err != nil {
		return err
	}
	if exists && (active || enabled) {
		if _, err := s.cmd.RunCommand(ctx, "systemctl", "disable", "--now", systemdScheduleName); err != nil {
			return fmt.Errorf("disabling systemd schedule: %w", err)
		}
	}
	if err := s.fs.Remove(systemdScheduleService); err != nil {
		return fmt.Errorf("removing systemd schedule service: %w", err)
	}
	if err := s.fs.Remove(systemdScheduleTimer); err != nil {
		return fmt.Errorf("removing systemd schedule timer: %w", err)
	}
	if _, err := s.cmd.RunCommand(ctx, "systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemd daemon-reload: %w", err)
	}
	return nil
}

func (s *linuxPlatformScheduler) timerState(ctx context.Context) (active bool, exists bool, enabled bool, err error) {
	out, err := s.cmd.RunCommand(ctx, "systemctl", "show", systemdScheduleName, "--property=ActiveState,LoadState,UnitFileState")
	if err != nil {
		return false, false, false, fmt.Errorf("querying systemd schedule: %w", err)
	}
	states := map[string]string{}
	for _, line := range strings.Split(out, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if ok {
			states[key] = value
		}
	}
	loadState := states["LoadState"]
	activeState := states["ActiveState"]
	if loadState == "" || activeState == "" {
		return false, false, false, fmt.Errorf("invalid systemd schedule state: %q", out)
	}
	if loadState == "not-found" {
		return false, false, false, nil
	}
	unitFileState := states["UnitFileState"]
	return activeState == "active", true, unitFileState == "enabled" || unitFileState == "enabled-runtime", nil
}

func (s *linuxPlatformScheduler) Status(ctx context.Context) (time.Duration, bool, error) {
	active, _, _, err := s.timerState(ctx)
	if err != nil {
		return 0, false, err
	}
	if !active {
		return 0, false, nil
	}
	data, err := s.fs.ReadFile(systemdScheduleTimer)
	if err != nil {
		return 0, false, fmt.Errorf("reading systemd schedule timer: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		const prefix = "OnUnitActiveSec="
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		seconds, err := strconv.ParseInt(strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(line, prefix)), "s"), 10, 64)
		if err != nil {
			return 0, false, fmt.Errorf("invalid systemd schedule interval: %w", err)
		}
		return time.Duration(seconds) * time.Second, true, nil
	}
	return 0, false, fmt.Errorf("systemd schedule interval is missing")
}

type LegacyScheduleError struct {
	Interval time.Duration
}

func (e LegacyScheduleError) Error() string {
	return fmt.Sprintf("legacy schedule configured for every %v; run subscription schedule --interval %v to activate it", e.Interval, e.Interval)
}

type nativeScheduleManager struct {
	fs          FileSystem
	platform    PlatformScheduler
	commandPath string
}

func NewScheduleManagerWithPlatform(fs FileSystem, platform PlatformScheduler, commandPath string) ScheduleManager {
	return &nativeScheduleManager{fs: fs, platform: platform, commandPath: commandPath}
}

func installedManagerPath() string {
	const releasePath = "/usr/local/bin/mihomo-manager"
	const legacyPath = "/opt/mihomo-manager/bin/mihomo-manager"
	executable, err := os.Executable()
	if err == nil {
		path := filepath.Clean(executable)
		if path == releasePath || path == legacyPath {
			return path
		}
	}
	return releasePath
}

func NewNativeScheduleManager(fs FileSystem, cmd CommandRunner) ScheduleManager {
	commandPath := installedManagerPath()
	switch runtime.GOOS {
	case "linux":
		return NewScheduleManagerWithPlatform(fs, NewLinuxPlatformScheduler(fs, cmd), commandPath)
	case "darwin":
		return NewScheduleManagerWithPlatform(fs, NewDarwinPlatformScheduler(fs, cmd), commandPath)
	default:
		return NewScheduleManagerWithPlatform(fs, unsupportedPlatformScheduler{os: runtime.GOOS}, commandPath)
	}
}

func (m *nativeScheduleManager) SetSchedule(ctx context.Context, interval time.Duration) error {
	if !m.fs.FileExists(binaryPath) {
		return fmt.Errorf("mihomo is not installed")
	}
	return m.platform.Set(ctx, interval, m.commandPath)
}

func (m *nativeScheduleManager) StopSchedule(ctx context.Context) error {
	if err := m.platform.Stop(ctx); err != nil {
		return err
	}
	return m.fs.WriteFile(scheduleFile, []byte("off"), filePermUserRW)
}

func (m *nativeScheduleManager) ScheduleStatus(ctx context.Context) (time.Duration, bool, error) {
	interval, active, err := m.platform.Status(ctx)
	if err != nil {
		return 0, false, err
	}
	if active {
		return interval, true, nil
	}
	data, readErr := m.fs.ReadFile(scheduleFile)
	if readErr == nil {
		raw := strings.TrimSpace(string(data))
		if raw != "" && raw != "off" {
			seconds, parseErr := strconv.ParseInt(raw, 10, 64)
			if parseErr == nil && seconds > 0 {
				return 0, false, LegacyScheduleError{Interval: time.Duration(seconds) * time.Second}
			}
		}
	}
	return 0, false, nil
}

type darwinPlatformScheduler struct {
	fs  FileSystem
	cmd CommandRunner
}

func (s *darwinPlatformScheduler) Set(ctx context.Context, interval time.Duration, commandPath string) error {
	if interval < time.Hour {
		return fmt.Errorf("minimum interval is 1h, got %v", interval)
	}
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>%s</string>
  <key>ProgramArguments</key>
  <array>
    <string>%s</string>
    <string>subscription</string>
    <string>update</string>
    <string>--quiet</string>
  </array>
  <key>StartInterval</key>
  <integer>%d</integer>
</dict>
</plist>
`, launchdScheduleLabel, commandPath, int64(interval.Seconds()))
	plist = strings.Replace(plist, "</dict>\n</plist>", "  <key>StandardOutPath</key>\n  <string>/var/log/mihomo-manager-subscription-update.log</string>\n  <key>StandardErrorPath</key>\n  <string>/var/log/mihomo-manager-subscription-update.err.log</string>\n</dict>\n</plist>", 1)
	if s.fs.FileExists(launchdSchedulePlist) {
		if err := s.bootout(ctx); err != nil {
			return err
		}
	}
	if err := s.fs.WriteFile(launchdSchedulePlist, []byte(plist), filePermUserRW); err != nil {
		return fmt.Errorf("writing launchd schedule: %w", err)
	}
	if _, err := s.cmd.RunCommand(ctx, "launchctl", "bootstrap", "system", launchdSchedulePlist); err != nil {
		return fmt.Errorf("loading launchd schedule: %w", err)
	}
	return nil
}

func (s *darwinPlatformScheduler) Stop(ctx context.Context) error {
	if s.fs.FileExists(launchdSchedulePlist) {
		if err := s.bootout(ctx); err != nil {
			return err
		}
	}
	return s.fs.Remove(launchdSchedulePlist)
}

func (s *darwinPlatformScheduler) bootout(ctx context.Context) error {
	output, err := s.cmd.RunCommand(ctx, "launchctl", "bootout", "system", launchdSchedulePlist)
	if err == nil || isLaunchdJobNotLoadedError(output, err) {
		return nil
	}
	return fmt.Errorf("unloading launchd schedule: %w", err)
}

func isLaunchdJobNotLoadedError(output string, err error) bool {
	if err == nil {
		return false
	}
	diagnostics := []string{output, err.Error()}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		diagnostics = append(diagnostics, string(exitErr.Stderr))
	}
	for _, diagnostic := range diagnostics {
		for _, line := range strings.Split(strings.ToLower(diagnostic), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "could not find service") ||
				strings.HasPrefix(line, "could not find specified service") ||
				strings.HasPrefix(line, "boot-out failed: 3: no such process") ||
				line == "service not found" || line == "job not found" || line == "job not loaded" {
				return true
			}
		}
	}
	return false
}

func (s *darwinPlatformScheduler) Status(ctx context.Context) (time.Duration, bool, error) {
	data, err := s.fs.ReadFile(launchdSchedulePlist)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	out, err := s.cmd.RunCommand(ctx, "launchctl", "print", "system/"+launchdScheduleLabel)
	if err != nil {
		return 0, false, fmt.Errorf("querying launchd schedule: %w", err)
	}
	active := strings.TrimSpace(out) != ""
	const prefix = "<integer>"
	const suffix = "</integer>"
	marker := "<key>StartInterval</key>"
	start := strings.Index(string(data), marker)
	if start < 0 {
		return 0, false, fmt.Errorf("launchd schedule interval is missing")
	}
	value := string(data)[start+len(marker):]
	open := strings.Index(value, prefix)
	close := strings.Index(value, suffix)
	if open < 0 || close < open {
		return 0, false, fmt.Errorf("invalid launchd schedule interval")
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(value[open+len(prefix):close]), 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid launchd schedule interval: %w", err)
	}
	return time.Duration(seconds) * time.Second, active, nil
}

type unsupportedPlatformScheduler struct{ os string }

func (s unsupportedPlatformScheduler) Set(context.Context, time.Duration, string) error {
	return fmt.Errorf("unsupported scheduler platform: %s", s.os)
}

func (s unsupportedPlatformScheduler) Stop(context.Context) error {
	return fmt.Errorf("unsupported scheduler platform: %s", s.os)
}

func (s unsupportedPlatformScheduler) Status(context.Context) (time.Duration, bool, error) {
	return 0, false, fmt.Errorf("unsupported scheduler platform: %s", s.os)
}
