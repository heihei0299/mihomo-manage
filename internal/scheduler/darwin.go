package scheduler

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	launchdSchedulePlist = "/Library/LaunchDaemons/mihomo-manager-subscription-update.plist"
	launchdScheduleLabel = "mihomo-manager-subscription-update"
)

// Darwin scheduler state is two-dimensional. The launchd runtime job and the
// plist on disk are independent and must never be inferred from one another.
//
// Runtime  Plist    Meaning
// loaded   present  normal active schedule
// loaded   missing  orphan runtime job; active with unknown persisted interval
// unloaded present  stale persisted configuration; inactive
// unloaded missing  fully stopped
//
// Set, Stop, and Status must preserve this matrix.
type darwinPlatform struct {
	fs  FileSystem
	cmd CommandRunner
}

func NewDarwin(fs FileSystem, cmd CommandRunner) Platform {
	return &darwinPlatform{fs: fs, cmd: cmd}
}

func (s *darwinPlatform) Set(ctx context.Context, interval time.Duration, commandPath string) error {
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
	loaded, err := s.isLoaded(ctx)
	if err != nil {
		return err
	}
	if loaded {
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

func (s *darwinPlatform) Stop(ctx context.Context) error {
	loaded, err := s.isLoaded(ctx)
	if err != nil {
		return err
	}
	if loaded {
		if err := s.bootout(ctx); err != nil {
			return err
		}
	}
	return s.fs.Remove(launchdSchedulePlist)
}

func (s *darwinPlatform) isLoaded(ctx context.Context) (bool, error) {
	output, err := s.cmd.RunCommand(ctx, "launchctl", "print", "system/"+launchdScheduleLabel)
	if err == nil {
		return true, nil
	}
	if isLaunchdJobNotLoadedError(output, err) {
		return false, nil
	}
	return false, fmt.Errorf("querying launchd schedule: %w", err)
}

func (s *darwinPlatform) bootout(ctx context.Context) error {
	output, err := s.cmd.RunCommand(ctx, "launchctl", "bootout", "system/"+launchdScheduleLabel)
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

func (s *darwinPlatform) Status(ctx context.Context) (time.Duration, bool, error) {
	loaded, err := s.isLoaded(ctx)
	if err != nil {
		return 0, false, err
	}
	data, err := s.fs.ReadFile(launchdSchedulePlist)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, loaded, nil
		}
		return 0, loaded, err
	}
	const prefix = "<integer>"
	const suffix = "</integer>"
	marker := "<key>StartInterval</key>"
	start := strings.Index(string(data), marker)
	if start < 0 {
		return 0, loaded, fmt.Errorf("launchd schedule interval is missing")
	}
	value := string(data)[start+len(marker):]
	open := strings.Index(value, prefix)
	close := strings.Index(value, suffix)
	if open < 0 || close < open {
		return 0, loaded, fmt.Errorf("invalid launchd schedule interval")
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(value[open+len(prefix):close]), 10, 64)
	if err != nil {
		return 0, loaded, fmt.Errorf("invalid launchd schedule interval: %w", err)
	}
	return time.Duration(seconds) * time.Second, loaded, nil
}
