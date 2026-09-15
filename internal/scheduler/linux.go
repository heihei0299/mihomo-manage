package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	systemdScheduleService = "/etc/systemd/system/mihomo-manager-subscription-update.service"
	systemdScheduleTimer   = "/etc/systemd/system/mihomo-manager-subscription-update.timer"
	systemdScheduleName    = "mihomo-manager-subscription-update.timer"
)

type linuxPlatform struct {
	fs  FileSystem
	cmd CommandRunner
}

func NewLinux(fs FileSystem, cmd CommandRunner) Platform {
	return &linuxPlatform{fs: fs, cmd: cmd}
}

func (s *linuxPlatform) Set(ctx context.Context, interval time.Duration, commandPath string) error {
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

func (s *linuxPlatform) Stop(ctx context.Context) error {
	state, err := s.timerState(ctx)
	if err != nil {
		return err
	}
	if state.exists && (state.active || state.enabled) {
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

type systemdTimerState struct {
	active  bool
	exists  bool
	enabled bool
}

func (s *linuxPlatform) timerState(ctx context.Context) (systemdTimerState, error) {
	out, err := s.cmd.RunCommand(ctx, "systemctl", "show", systemdScheduleName, "--property=ActiveState,LoadState,UnitFileState")
	if err != nil {
		return systemdTimerState{}, fmt.Errorf("querying systemd schedule: %w", err)
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
		return systemdTimerState{}, fmt.Errorf("invalid systemd schedule state: %q", out)
	}
	if loadState == "not-found" {
		return systemdTimerState{}, nil
	}
	unitFileState := states["UnitFileState"]
	return systemdTimerState{
		active:  activeState == "active",
		exists:  true,
		enabled: unitFileState == "enabled" || unitFileState == "enabled-runtime",
	}, nil
}

func (s *linuxPlatform) Status(ctx context.Context) (time.Duration, bool, error) {
	state, err := s.timerState(ctx)
	if err != nil {
		return 0, false, err
	}
	if !state.active {
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
