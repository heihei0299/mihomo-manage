package scheduler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLinuxPlatformWritesNativeUnits(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &commandRecorder{}
	platform := NewLinux(fs, cmd)

	if err := platform.Set(context.Background(), 2*time.Hour, "/opt/mihomo-manager/bin/mihomo-manager"); err != nil {
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

func TestLinuxPlatformStatusPropagatesQueryFailure(t *testing.T) {
	platform := NewLinux(&fakeFileSystem{}, &commandRecorder{cmdErr: errors.New("systemd unavailable")})
	if _, _, err := platform.Status(context.Background()); err == nil {
		t.Fatal("Status should report systemd query failure")
	}
}

func TestLinuxPlatformStopRemovesUnits(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{
		systemdScheduleService: []byte("service"),
		systemdScheduleTimer:   []byte("timer"),
	}}
	platform := NewLinux(fs, &commandRecorder{output: "LoadState=not-found\nActiveState=inactive\nUnitFileState="})

	if err := platform.Stop(context.Background()); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if _, ok := fs.written[systemdScheduleService]; ok {
		t.Fatal("service unit should be removed")
	}
	if _, ok := fs.written[systemdScheduleTimer]; ok {
		t.Fatal("timer unit should be removed")
	}
}

func TestLinuxPlatformStatusReadsNativeTimer(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{
		systemdScheduleTimer: []byte("OnUnitActiveSec=3600s\nPersistent=false\n"),
	}}
	platform := NewLinux(fs, &commandRecorder{output: "LoadState=loaded\nActiveState=active\nUnitFileState=enabled"})

	interval, active, err := platform.Status(context.Background())
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if !active || interval != time.Hour {
		t.Fatalf("status = %v, %v; want 1h active", interval, active)
	}
}
