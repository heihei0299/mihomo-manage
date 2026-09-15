package scheduler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestDarwinPlatformSetStopsLoadedJobWithoutPlist(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &commandRecorder{responses: []commandResponse{{output: "loaded"}, {}, {}}}
	platform := NewDarwin(fs, cmd)

	if err := platform.Set(context.Background(), 2*time.Hour, "/usr/local/bin/mihomo-manager"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if len(cmd.captured) != 3 || cmd.captured[0].args[0] != "print" || cmd.captured[1].args[0] != "bootout" || cmd.captured[2].args[0] != "bootstrap" {
		t.Fatalf("commands = %v, want print, bootout, bootstrap", cmd.captured)
	}
	if got := cmd.captured[1].args[1]; got != "system/"+launchdScheduleLabel {
		t.Fatalf("bootout target = %q, want service label", got)
	}
}

func TestDarwinPlatformSetIsRepeatable(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{launchdSchedulePlist: []byte("old plist")}}
	cmd := &commandRecorder{responses: []commandResponse{
		{output: "loaded"}, {}, {},
		{output: "loaded"}, {}, {},
	}}
	platform := NewDarwin(fs, cmd)

	if err := platform.Set(context.Background(), 2*time.Hour, "/usr/local/bin/mihomo-manager"); err != nil {
		t.Fatalf("first Set failed: %v", err)
	}
	if err := platform.Set(context.Background(), 3*time.Hour, "/usr/local/bin/mihomo-manager"); err != nil {
		t.Fatalf("second Set failed: %v", err)
	}
	plist := string(fs.written[launchdSchedulePlist])
	if !strings.Contains(plist, "<string>/usr/local/bin/mihomo-manager</string>") || !strings.Contains(plist, "<integer>10800</integer>") {
		t.Fatalf("plist = %q, missing command or updated interval", plist)
	}
}

func TestDarwinPlatformSetLeavesUnloadedJobWithoutPlist(t *testing.T) {
	cmd := &commandRecorder{responses: []commandResponse{
		{output: "Could not find service", err: errors.New("exit status 3")},
		{},
	}}
	platform := NewDarwin(&fakeFileSystem{}, cmd)

	if err := platform.Set(context.Background(), 3*time.Hour, "/usr/local/bin/mihomo-manager"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if len(cmd.captured) != 2 || cmd.captured[0].args[0] != "print" || cmd.captured[1].args[0] != "bootstrap" {
		t.Fatalf("commands = %v, want print then bootstrap", cmd.captured)
	}
}

func TestDarwinPlatformSetRecoversFromUnloadedJob(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{launchdSchedulePlist: []byte("stale plist")}}
	cmd := &commandRecorder{responses: []commandResponse{
		{output: "Could not find service", err: errors.New("exit status 3")},
		{},
	}}
	platform := NewDarwin(fs, cmd)

	if err := platform.Set(context.Background(), 3*time.Hour, "/usr/local/bin/mihomo-manager"); err != nil {
		t.Fatalf("Set should recover an unloaded job: %v", err)
	}
	if !strings.Contains(string(fs.written[launchdSchedulePlist]), "<integer>10800</integer>") {
		t.Fatalf("plist = %q, want updated interval", fs.written[launchdSchedulePlist])
	}
}

func TestDarwinPlatformSetPropagatesBootoutFailure(t *testing.T) {
	const oldPlist = "old plist"
	fs := &fakeFileSystem{written: map[string][]byte{launchdSchedulePlist: []byte(oldPlist)}}
	cmd := &commandRecorder{responses: []commandResponse{
		{},
		{output: "Operation not permitted", err: errors.New("exit status 1")},
	}}
	platform := NewDarwin(fs, cmd)

	err := platform.Set(context.Background(), 3*time.Hour, "/usr/local/bin/mihomo-manager")
	if err == nil || !strings.Contains(err.Error(), "unloading launchd schedule") {
		t.Fatalf("Set error = %v, want bootout failure", err)
	}
	if string(fs.written[launchdSchedulePlist]) != oldPlist {
		t.Fatal("plist should not be rewritten when bootout fails")
	}
}

func TestDarwinPlatformSetToleratesJobDisappearingBeforeBootout(t *testing.T) {
	cmd := &commandRecorder{responses: []commandResponse{
		{},
		{output: "Boot-out failed: 3: No such process", err: errors.New("exit status 3")},
		{},
	}}
	platform := NewDarwin(&fakeFileSystem{}, cmd)
	if err := platform.Set(context.Background(), 3*time.Hour, "/usr/local/bin/mihomo-manager"); err != nil {
		t.Fatalf("Set should tolerate a disappearing job: %v", err)
	}
}

func TestDarwinPlatformPropagatesPrintFailure(t *testing.T) {
	cmd := &commandRecorder{responses: []commandResponse{{output: "Operation not permitted", err: errors.New("exit status 1")}}}
	platform := NewDarwin(&fakeFileSystem{}, cmd)
	if err := platform.Stop(context.Background()); err == nil || !strings.Contains(err.Error(), "querying launchd schedule") {
		t.Fatalf("Stop error = %v, want launchd print failure", err)
	}
}

func TestDarwinPlatformStatusMatrix(t *testing.T) {
	tests := []struct {
		name     string
		fs       *fakeFileSystem
		cmd      *commandRecorder
		interval time.Duration
		active   bool
	}{
		{
			name:   "loaded missing plist",
			fs:     &fakeFileSystem{},
			cmd:    &commandRecorder{responses: []commandResponse{{output: "loaded"}}},
			active: true,
		},
		{
			name: "unloaded missing plist",
			fs:   &fakeFileSystem{},
			cmd:  &commandRecorder{responses: []commandResponse{{output: "Could not find service", err: errors.New("exit status 3")}}},
		},
		{
			name:     "unloaded existing plist",
			fs:       &fakeFileSystem{written: map[string][]byte{launchdSchedulePlist: []byte("<key>StartInterval</key><integer>3600</integer>")}},
			cmd:      &commandRecorder{responses: []commandResponse{{output: "Could not find service", err: errors.New("exit status 3")}}},
			interval: time.Hour,
		},
		{
			name:     "loaded existing plist",
			fs:       &fakeFileSystem{written: map[string][]byte{launchdSchedulePlist: []byte("<key>StartInterval</key><integer>3600</integer>")}},
			cmd:      &commandRecorder{output: "loaded"},
			interval: time.Hour,
			active:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platform := NewDarwin(tt.fs, tt.cmd)
			interval, active, err := platform.Status(context.Background())
			if err != nil {
				t.Fatalf("Status failed: %v", err)
			}
			if interval != tt.interval || active != tt.active {
				t.Fatalf("status = %v, %v; want %v, %v", interval, active, tt.interval, tt.active)
			}
		})
	}
}

func TestDarwinPlatformStatusPropagatesQueryFailure(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{launchdSchedulePlist: []byte("<key>StartInterval</key><integer>3600</integer>")}}
	platform := NewDarwin(fs, &commandRecorder{cmdErr: errors.New("launchd unavailable")})
	if _, _, err := platform.Status(context.Background()); err == nil {
		t.Fatal("Status should report launchd query failure")
	}
}

func TestDarwinPlatformStopIsRepeatable(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{launchdSchedulePlist: []byte("plist")}}
	cmd := &commandRecorder{responses: []commandResponse{
		{}, {},
		{output: "Could not find service", err: errors.New("exit status 3")},
	}}
	platform := NewDarwin(fs, cmd)

	if err := platform.Stop(context.Background()); err != nil {
		t.Fatalf("first Stop failed: %v", err)
	}
	if err := platform.Stop(context.Background()); err != nil {
		t.Fatalf("second Stop failed: %v", err)
	}
	if _, ok := fs.written[launchdSchedulePlist]; ok {
		t.Fatal("launchd plist should be removed")
	}
}

func TestDarwinPlatformStopRemovesUnloadedPlist(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{launchdSchedulePlist: []byte("stale plist")}}
	cmd := &commandRecorder{responses: []commandResponse{{output: "Could not find service", err: errors.New("exit status 3")}}}
	platform := NewDarwin(fs, cmd)

	if err := platform.Stop(context.Background()); err != nil {
		t.Fatalf("Stop should remove an unloaded plist: %v", err)
	}
	if _, ok := fs.written[launchdSchedulePlist]; ok {
		t.Fatal("stale launchd plist should be removed")
	}
}

func TestDarwinPlatformStopPropagatesBootoutFailure(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{launchdSchedulePlist: []byte("plist")}}
	cmd := &commandRecorder{responses: []commandResponse{
		{},
		{output: "Operation not permitted", err: errors.New("exit status 1")},
	}}
	platform := NewDarwin(fs, cmd)

	err := platform.Stop(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unloading launchd schedule") {
		t.Fatalf("Stop error = %v, want bootout failure", err)
	}
	if _, ok := fs.written[launchdSchedulePlist]; !ok {
		t.Fatal("plist should remain when bootout fails")
	}
}

func TestDarwinPlatformStopDoesNotIgnoreSimilarFailure(t *testing.T) {
	fs := &fakeFileSystem{written: map[string][]byte{launchdSchedulePlist: []byte("plist")}}
	cmd := &commandRecorder{responses: []commandResponse{
		{},
		{output: "permission denied: service not found", err: errors.New("exit status 1")},
	}}
	platform := NewDarwin(fs, cmd)
	if err := platform.Stop(context.Background()); err == nil {
		t.Fatal("Stop should not ignore an unrelated not-found phrase")
	}
}
