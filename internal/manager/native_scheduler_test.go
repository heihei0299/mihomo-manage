package manager

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

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

func TestScheduleSetAndStop(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{binaryPath: true}}
	m := NewScheduleManagerWithPlatform(fs, &fakePlatformScheduler{}, "/opt/mihomo-manager/bin/mihomo-manager")

	if err := m.SetSchedule(context.Background(), time.Hour); err != nil {
		t.Fatalf("SetSchedule failed: %v", err)
	}
	interval, active, err := m.ScheduleStatus(context.Background())
	if err != nil {
		t.Fatalf("ScheduleStatus failed: %v", err)
	}
	if !active || interval != time.Hour {
		t.Fatalf("status = %v, %v; want 1h active", interval, active)
	}
	if err := m.StopSchedule(context.Background()); err != nil {
		t.Fatalf("StopSchedule failed: %v", err)
	}
	_, active, err = m.ScheduleStatus(context.Background())
	if err != nil {
		t.Fatalf("ScheduleStatus after stop failed: %v", err)
	}
	if active {
		t.Fatal("expected schedule to be inactive after stop")
	}
}

func TestScheduleRejectsShortInterval(t *testing.T) {
	fs := &fakeFileSystem{fileExists: map[string]bool{binaryPath: true}}
	m := NewScheduleManagerWithPlatform(fs, &fakePlatformScheduler{}, "/opt/mihomo-manager/bin/mihomo-manager")
	if err := m.SetSchedule(context.Background(), time.Minute); err == nil {
		t.Fatal("expected error for interval < 1h")
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
