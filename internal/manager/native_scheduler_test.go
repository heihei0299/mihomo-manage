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

func TestDarwinPlatformSchedulerSetStopsLoadedJobWithoutPlist(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &commandRecorder{responses: []commandResponse{{output: "loaded"}, {}, {}}}
	scheduler := NewDarwinPlatformScheduler(fs, cmd)

	if err := scheduler.Set(context.Background(), 2*time.Hour, "/usr/local/bin/mihomo-manager"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if len(cmd.captured) != 3 || cmd.captured[0].args[0] != "print" || cmd.captured[1].args[0] != "bootout" || cmd.captured[2].args[0] != "bootstrap" {
		t.Fatalf("commands = %v, want print, bootout, bootstrap", cmd.captured)
	}
}

func TestDarwinPlatformSchedulerSetIsRepeatable(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{launchdSchedulePlist: true},
		written:    map[string][]byte{launchdSchedulePlist: []byte("old plist")},
	}
	cmd := &commandRecorder{responses: []commandResponse{
		{output: "loaded"}, {}, {},
		{output: "loaded"}, {}, {},
	}}
	scheduler := NewDarwinPlatformScheduler(fs, cmd)

	if err := scheduler.Set(context.Background(), 2*time.Hour, "/usr/local/bin/mihomo-manager"); err != nil {
		t.Fatalf("first Set failed: %v", err)
	}
	if err := scheduler.Set(context.Background(), 3*time.Hour, "/usr/local/bin/mihomo-manager"); err != nil {
		t.Fatalf("second Set failed: %v", err)
	}
	plist := string(fs.written[launchdSchedulePlist])
	if !strings.Contains(plist, "<string>/usr/local/bin/mihomo-manager</string>") || !strings.Contains(plist, "<integer>10800</integer>") {
		t.Fatalf("plist = %q, missing command or updated interval", plist)
	}
	if len(cmd.captured) != 6 || cmd.captured[0].args[0] != "print" || cmd.captured[1].args[0] != "bootout" || cmd.captured[2].args[0] != "bootstrap" || cmd.captured[3].args[0] != "print" || cmd.captured[4].args[0] != "bootout" || cmd.captured[5].args[0] != "bootstrap" {
		t.Fatalf("commands = %v, want print, bootout, bootstrap twice", cmd.captured)
	}
}

func TestDarwinPlatformSchedulerSetLeavesUnloadedJobWithoutPlist(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &commandRecorder{responses: []commandResponse{
		{output: "Could not find service", err: errors.New("exit status 3")},
		{},
	}}
	scheduler := NewDarwinPlatformScheduler(fs, cmd)

	if err := scheduler.Set(context.Background(), 3*time.Hour, "/usr/local/bin/mihomo-manager"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if len(cmd.captured) != 2 || cmd.captured[0].args[0] != "print" || cmd.captured[1].args[0] != "bootstrap" {
		t.Fatalf("commands = %v, want print then bootstrap", cmd.captured)
	}
}

func TestDarwinPlatformSchedulerSetRecoversFromUnloadedJob(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{launchdSchedulePlist: true},
		written:    map[string][]byte{launchdSchedulePlist: []byte("stale plist")},
	}
	cmd := &commandRecorder{responses: []commandResponse{
		{output: "Could not find service", err: errors.New("exit status 3")},
		{},
	}}
	scheduler := NewDarwinPlatformScheduler(fs, cmd)

	if err := scheduler.Set(context.Background(), 3*time.Hour, "/usr/local/bin/mihomo-manager"); err != nil {
		t.Fatalf("Set should recover an unloaded job: %v", err)
	}
	if !strings.Contains(string(fs.written[launchdSchedulePlist]), "<integer>10800</integer>") {
		t.Fatalf("plist = %q, want updated interval", fs.written[launchdSchedulePlist])
	}
	if len(cmd.captured) != 2 || cmd.captured[0].args[0] != "print" || cmd.captured[1].args[0] != "bootstrap" {
		t.Fatalf("commands = %v, want print then bootstrap", cmd.captured)
	}
}

func TestDarwinPlatformSchedulerSetPropagatesBootoutFailure(t *testing.T) {
	const oldPlist = "old plist"
	fs := &fakeFileSystem{
		fileExists: map[string]bool{launchdSchedulePlist: true},
		written:    map[string][]byte{launchdSchedulePlist: []byte(oldPlist)},
	}
	cmd := &commandRecorder{responses: []commandResponse{
		{},
		{output: "Operation not permitted", err: errors.New("exit status 1")},
	}}
	scheduler := NewDarwinPlatformScheduler(fs, cmd)

	err := scheduler.Set(context.Background(), 3*time.Hour, "/usr/local/bin/mihomo-manager")
	if err == nil || !strings.Contains(err.Error(), "unloading launchd schedule") {
		t.Fatalf("Set error = %v, want bootout failure", err)
	}
	if string(fs.written[launchdSchedulePlist]) != oldPlist {
		t.Fatal("plist should not be rewritten when bootout fails")
	}
}

func TestDarwinPlatformSchedulerSetToleratesJobDisappearingBeforeBootout(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &commandRecorder{responses: []commandResponse{
		{},
		{output: "Boot-out failed: 3: No such process", err: errors.New("exit status 3")},
		{},
	}}
	scheduler := NewDarwinPlatformScheduler(fs, cmd)

	if err := scheduler.Set(context.Background(), 3*time.Hour, "/usr/local/bin/mihomo-manager"); err != nil {
		t.Fatalf("Set should tolerate a disappearing job: %v", err)
	}
}

func TestDarwinPlatformSchedulerPropagatesPrintFailure(t *testing.T) {
	cmd := &commandRecorder{responses: []commandResponse{{output: "Operation not permitted", err: errors.New("exit status 1")}}}
	scheduler := NewDarwinPlatformScheduler(&fakeFileSystem{}, cmd)

	if err := scheduler.Stop(context.Background()); err == nil || !strings.Contains(err.Error(), "querying launchd schedule") {
		t.Fatalf("Stop error = %v, want launchd print failure", err)
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

func TestDarwinPlatformSchedulerStopStopsLoadedJobWithoutPlist(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &commandRecorder{responses: []commandResponse{{output: "loaded"}, {}}}
	scheduler := NewDarwinPlatformScheduler(fs, cmd)

	if err := scheduler.Stop(context.Background()); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if len(cmd.captured) != 2 || cmd.captured[0].args[0] != "print" || cmd.captured[1].args[0] != "bootout" {
		t.Fatalf("commands = %v, want print, bootout", cmd.captured)
	}
}

func TestDarwinPlatformSchedulerStopIsRepeatable(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{launchdSchedulePlist: true},
		written:    map[string][]byte{launchdSchedulePlist: []byte("plist")},
	}
	cmd := &commandRecorder{responses: []commandResponse{
		{}, {},
		{output: "Could not find service", err: errors.New("exit status 3")},
	}}
	scheduler := NewDarwinPlatformScheduler(fs, cmd)

	if err := scheduler.Stop(context.Background()); err != nil {
		t.Fatalf("first Stop failed: %v", err)
	}
	if err := scheduler.Stop(context.Background()); err != nil {
		t.Fatalf("second Stop failed: %v", err)
	}
	if _, ok := fs.written[launchdSchedulePlist]; ok {
		t.Fatal("launchd plist should be removed")
	}
	if len(cmd.captured) != 3 || cmd.captured[0].args[0] != "print" || cmd.captured[1].args[0] != "bootout" || cmd.captured[2].args[0] != "print" {
		t.Fatalf("commands = %v, want print, bootout, print", cmd.captured)
	}
}

func TestDarwinPlatformSchedulerStopRemovesUnloadedPlist(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{launchdSchedulePlist: true},
		written:    map[string][]byte{launchdSchedulePlist: []byte("stale plist")},
	}
	cmd := &commandRecorder{responses: []commandResponse{{output: "Could not find service", err: errors.New("exit status 3")}}}
	scheduler := NewDarwinPlatformScheduler(fs, cmd)

	if err := scheduler.Stop(context.Background()); err != nil {
		t.Fatalf("Stop should remove an unloaded plist: %v", err)
	}
	if _, ok := fs.written[launchdSchedulePlist]; ok {
		t.Fatal("stale launchd plist should be removed")
	}
	if len(cmd.captured) != 1 || cmd.captured[0].args[0] != "print" {
		t.Fatalf("commands = %v, want print only", cmd.captured)
	}
}

func TestDarwinPlatformSchedulerStopWithoutPlist(t *testing.T) {
	cmd := &commandRecorder{responses: []commandResponse{{output: "Could not find service", err: errors.New("exit status 3")}}}
	scheduler := NewDarwinPlatformScheduler(&fakeFileSystem{}, cmd)

	if err := scheduler.Stop(context.Background()); err != nil {
		t.Fatalf("Stop without a plist should succeed: %v", err)
	}
	if len(cmd.captured) != 1 || cmd.captured[0].args[0] != "print" {
		t.Fatalf("commands = %v, want print only", cmd.captured)
	}
}

func TestDarwinPlatformSchedulerStopPropagatesBootoutFailure(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{launchdSchedulePlist: true},
		written:    map[string][]byte{launchdSchedulePlist: []byte("plist")},
	}
	cmd := &commandRecorder{responses: []commandResponse{
		{},
		{output: "Operation not permitted", err: errors.New("exit status 1")},
	}}
	scheduler := NewDarwinPlatformScheduler(fs, cmd)

	err := scheduler.Stop(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unloading launchd schedule") {
		t.Fatalf("Stop error = %v, want bootout failure", err)
	}
	if _, ok := fs.written[launchdSchedulePlist]; !ok {
		t.Fatal("plist should remain when bootout fails")
	}
}

func TestDarwinPlatformSchedulerStopDoesNotIgnoreSimilarFailure(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{launchdSchedulePlist: true},
		written:    map[string][]byte{launchdSchedulePlist: []byte("plist")},
	}
	cmd := &commandRecorder{responses: []commandResponse{
		{},
		{output: "permission denied: service not found", err: errors.New("exit status 1")},
	}}
	scheduler := NewDarwinPlatformScheduler(fs, cmd)

	if err := scheduler.Stop(context.Background()); err == nil {
		t.Fatal("Stop should not ignore an unrelated not-found phrase")
	}
	if _, ok := fs.written[launchdSchedulePlist]; !ok {
		t.Fatal("plist should remain when bootout fails")
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
