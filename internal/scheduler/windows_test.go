package scheduler

import (
	"context"
	"encoding/xml"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestWindowsScheduleRunsSystemTaskWithEscapedExecutable(t *testing.T) {
	fs := &fakeFileSystem{}
	cmd := &commandRecorder{}
	platform := NewWindows(fs, cmd, "task.xml")
	const commandPath = `C:\Program Files\A&B\mihomo-manager.exe`
	if err := platform.Set(context.Background(), 90*time.Minute, commandPath); err != nil {
		t.Fatal(err)
	}
	var task struct {
		User     string `xml:"Principals>Principal>UserId"`
		Interval string `xml:"Triggers>TimeTrigger>Repetition>Interval"`
		Command  string `xml:"Actions>Exec>Command"`
		Args     string `xml:"Actions>Exec>Arguments"`
	}
	if err := xml.Unmarshal(fs.written["task.xml"], &task); err != nil {
		t.Fatal(err)
	}
	if task.User != "S-1-5-18" || task.Command != commandPath || task.Interval != "PT5400S" || task.Args != "subscription update --quiet" {
		t.Fatalf("task = %+v", task)
	}
	if len(cmd.captured) != 1 || cmd.captured[0].name != "schtasks.exe" {
		t.Fatalf("commands = %+v", cmd.captured)
	}
}

func TestWindowsScheduleRejectsUnsupportedIntervalsBeforeWriting(t *testing.T) {
	for _, interval := range []time.Duration{0, time.Minute, 32 * 24 * time.Hour, time.Hour + time.Millisecond} {
		fs, cmd := &fakeFileSystem{}, &commandRecorder{}
		if err := NewWindows(fs, cmd, "task.xml").Set(context.Background(), interval, "manager.exe"); err == nil || len(fs.written) != 0 || len(cmd.captured) != 0 {
			t.Fatalf("interval %v: error=%v files=%v commands=%v", interval, err, fs.written, cmd.captured)
		}
	}
}

func TestWindowsScheduleStatusUsesRegisteredTask(t *testing.T) {
	for _, test := range []struct {
		name, output string
		active       bool
		interval     time.Duration
	}{
		{"registered without staging file", `<Task><Settings><Enabled>true</Enabled></Settings><Triggers><TimeTrigger><Repetition><Interval>PT1H30M</Interval></Repetition></TimeTrigger></Triggers></Task>`, true, 90 * time.Minute},
		{"absent with stale staging file", "MISSING", false, 0},
		{"disabled", `<Task><Settings><Enabled>false</Enabled></Settings></Task>`, false, 0},
		{"disabled trigger", `<Task><Triggers><TimeTrigger><Enabled>false</Enabled></TimeTrigger></Triggers></Task>`, false, 0},
		{"COM Unicode declaration", `<?xml version="1.0" encoding="UTF-16"?><Task><Triggers><TimeTrigger><Repetition><Interval>PT2H</Interval></Repetition></TimeTrigger></Triggers></Task>`, true, 2 * time.Hour},
	} {
		t.Run(test.name, func(t *testing.T) {
			fs := &fakeFileSystem{written: map[string][]byte{"task.xml": []byte("stale")}}
			if test.active {
				fs.written = nil
			}
			interval, active, err := NewWindows(fs, &commandRecorder{output: test.output}, "task.xml").Status(context.Background())
			if err != nil || active != test.active || interval != test.interval {
				t.Fatalf("status = %v, %v, %v", interval, active, err)
			}
		})
	}
}

func TestWindowsScheduleStopIsIdempotentAndPropagatesErrors(t *testing.T) {
	for _, output := range []string{"MISSING", "<Task/>"} {
		fs, cmd := &fakeFileSystem{written: map[string][]byte{"task.xml": []byte("old")}}, &commandRecorder{output: output}
		if err := NewWindows(fs, cmd, "task.xml").Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
		if _, exists := fs.written["task.xml"]; exists {
			t.Fatal("staging XML retained")
		}
		if output != "MISSING" && (len(cmd.captured) != 2 || !strings.Contains(strings.Join(cmd.captured[1].args, " "), ".Stop(0)")) {
			t.Fatal("running task was not stopped before deletion")
		}
	}
	wantErr := errors.New("task scheduler denied access")
	fs := &fakeFileSystem{written: map[string][]byte{"task.xml": []byte("old")}}
	cmd := &commandRecorder{responses: []commandResponse{{output: "<Task/>"}, {err: wantErr}}}
	if err := NewWindows(fs, cmd, "task.xml").Stop(context.Background()); !errors.Is(err, wantErr) || len(fs.removed) != 0 {
		t.Fatalf("Stop = %v, removed = %v", err, fs.removed)
	}
}

func TestWindowsScheduleQueryFailureIsNotReportedAsOff(t *testing.T) {
	wantErr := errors.New("access denied")
	_, _, err := NewWindows(&fakeFileSystem{}, &commandRecorder{cmdErr: wantErr}, "task.xml").Status(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("Status = %v", err)
	}
}

func TestWindowsTaskIntervalParser(t *testing.T) {
	for raw, want := range map[string]time.Duration{"PT3600S": time.Hour, "PT1H30M": 90 * time.Minute, "P1D": 24 * time.Hour, "P31D": 31 * 24 * time.Hour} {
		if got, err := parseTaskInterval(raw); err != nil || got != want {
			t.Fatalf("%s: %v, %v", raw, got, err)
		}
	}
	for _, raw := range []string{"P", "PT0S", "PT-1H", "P32D", "PT9223372036854775807H", "PT1", "PT1X"} {
		if _, err := parseTaskInterval(raw); err == nil {
			t.Fatalf("accepted invalid interval %q", raw)
		}
	}
}
