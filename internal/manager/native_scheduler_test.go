package manager

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestLinuxPlatformSchedulerWritesNativeUnits(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &commandRecorder{}
	scheduler := NewLinuxPlatformScheduler(fs, cmd)

	if err := scheduler.Set(context.Background(), 2*time.Hour, "/opt/mihomo-manager/bin/mihomo-manager"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	service := string(fs.written[systemdScheduleService])
	timer := string(fs.written[systemdScheduleTimer])
	if !strings.Contains(service, "ExecStart=/opt/mihomo-manager/bin/mihomo-manager subscription update --quiet") {
		t.Fatalf("service = %q, missing scheduled command", service)
	}
	if !strings.Contains(timer, "OnUnitActiveSec=7200s") || !strings.Contains(timer, "Persistent=false") {
		t.Fatalf("timer = %q, missing interval semantics", timer)
	}
	if len(cmd.captured) != 2 || cmd.captured[0].args[0] != "daemon-reload" || cmd.captured[1].args[0] != "enable" {
		t.Fatalf("commands = %v, want daemon-reload then enable", cmd.captured)
	}
}

func TestLinuxPlatformSchedulerStatusPropagatesQueryFailure(t *testing.T) {
	cmd := &commandRecorder{cmdErr: errors.New("systemd unavailable")}
	scheduler := NewLinuxPlatformScheduler(&fakeFileSystem{}, cmd)

	if _, _, err := scheduler.Status(context.Background()); err == nil {
		t.Fatal("Status should report systemd query failure")
	}
}

func TestLinuxPlatformSchedulerStopRemovesUnits(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{
		systemdScheduleService: []byte("service"),
		systemdScheduleTimer:   []byte("timer"),
	}}
	scheduler := NewLinuxPlatformScheduler(fs, &commandRecorder{output: "LoadState=not-found\nActiveState=inactive\nUnitFileState="})

	if err := scheduler.Stop(context.Background()); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if _, ok := fs.written[systemdScheduleService]; ok {
		t.Fatal("service unit should be removed")
	}
	if _, ok := fs.written[systemdScheduleTimer]; ok {
		t.Fatal("timer unit should be removed")
	}
}

func TestLinuxPlatformSchedulerStatusReadsNativeTimer(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{
		systemdScheduleTimer: []byte("OnUnitActiveSec=3600s\nPersistent=false\n"),
	}}
	cmd := &commandRecorder{output: "LoadState=loaded\nActiveState=active\nUnitFileState=enabled"}
	scheduler := NewLinuxPlatformScheduler(fs, cmd)

	interval, active, err := scheduler.Status(context.Background())
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if !active || interval != time.Hour {
		t.Fatalf("status = %v, %v; want 1h active", interval, active)
	}
}

func TestDarwinPlatformSchedulerWritesLaunchdPlist(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &commandRecorder{}
	scheduler := NewDarwinPlatformScheduler(fs, cmd)

	if err := scheduler.Set(context.Background(), 2*time.Hour, "/usr/local/bin/mihomo-manager"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	plist := string(fs.written[launchdSchedulePlist])
	if !strings.Contains(plist, "<string>/usr/local/bin/mihomo-manager</string>") || !strings.Contains(plist, "<integer>7200</integer>") {
		t.Fatalf("plist = %q, missing command or interval", plist)
	}
	if len(cmd.captured) != 1 || cmd.captured[0].args[0] != "bootstrap" || cmd.captured[0].args[1] != "system" {
		t.Fatalf("commands = %v, want launchctl bootstrap system", cmd.captured)
	}
}

func TestDarwinPlatformSchedulerStatusReadsLaunchdPlist(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{launchdSchedulePlist: true},
		written: map[string][]byte{
			launchdSchedulePlist: []byte("<key>StartInterval</key><integer>3600</integer>"),
		},
	}
	scheduler := NewDarwinPlatformScheduler(fs, &commandRecorder{output: "loaded"})

	interval, active, err := scheduler.Status(context.Background())
	if err != nil || !active || interval != time.Hour {
		t.Fatalf("status = %v, %v, %v; want 1h active", interval, active, err)
	}
}

func TestDarwinPlatformSchedulerStatusPropagatesQueryFailure(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{launchdSchedulePlist: true},
		written:    map[string][]byte{launchdSchedulePlist: []byte("<key>StartInterval</key><integer>3600</integer>")},
	}
	scheduler := NewDarwinPlatformScheduler(fs, &commandRecorder{cmdErr: errors.New("launchd unavailable")})

	if _, _, err := scheduler.Status(context.Background()); err == nil {
		t.Fatal("Status should report launchd query failure")
	}
}

func TestDarwinPlatformSchedulerStopRemovesPlist(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{launchdSchedulePlist: true},
		written:    map[string][]byte{launchdSchedulePlist: []byte("plist")},
	}
	scheduler := NewDarwinPlatformScheduler(fs, &commandRecorder{})

	if err := scheduler.Stop(context.Background()); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if _, ok := fs.written[launchdSchedulePlist]; ok {
		t.Fatal("launchd plist should be removed")
	}
}

func TestNativeScheduleManagerReportsLegacySchedule(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{scheduleFile: []byte("7200")}}
	platform := &fakePlatformScheduler{}
	m := NewScheduleManagerWithPlatform(fs, platform, "/opt/mihomo-manager/bin/mihomo-manager")

	_, active, err := m.ScheduleStatus(context.Background())
	if active {
		t.Fatal("legacy schedule must not be reported as active native schedule")
	}
	var legacy LegacyScheduleError
	if !errors.As(err, &legacy) {
		t.Fatalf("ScheduleStatus error = %v, want legacy schedule error", err)
	}
}

type fakePlatformScheduler struct {
	interval time.Duration
	active   bool
}

func (s *fakePlatformScheduler) Set(_ context.Context, interval time.Duration, _ string) error {
	if interval < time.Hour {
		return fmt.Errorf("minimum interval is 1h")
	}
	s.interval = interval
	s.active = true
	return nil
}

func (s *fakePlatformScheduler) Stop(context.Context) error {
	s.active = false
	return nil
}

func (s *fakePlatformScheduler) Status(context.Context) (time.Duration, bool, error) {
	return s.interval, s.active, nil
}
