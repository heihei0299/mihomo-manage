package manager

import (
	"context"
	"fmt"
	"testing"
)

type commandRecorder struct {
	captured  []cmdCall
	output    string
	cmdErr    error
	responses []commandResponse
	lastCtx   context.Context
}

type cmdCall struct {
	name string
	args []string
}

type commandResponse struct {
	output string
	err    error
}

func (r *commandRecorder) RunCommand(ctx context.Context, name string, args ...string) (string, error) {
	r.lastCtx = ctx
	r.captured = append(r.captured, cmdCall{name, args})
	if len(r.responses) > 0 {
		response := r.responses[0]
		r.responses = r.responses[1:]
		return response.output, response.err
	}
	if r.cmdErr != nil {
		return "inactive", r.cmdErr
	}
	if r.output != "" {
		return r.output, nil
	}
	return "active", nil
}

func (r *commandRecorder) RunCommandIgnoreExit(ctx context.Context, name string, args ...string) (string, error) {
	r.lastCtx = ctx
	r.captured = append(r.captured, cmdCall{name, args})
	if r.output != "" {
		return r.output, nil
	}
	return "inactive", nil
}

func TestOSServiceManagerPassesContextToCommands(t *testing.T) {
	rec := &commandRecorder{}
	svc := &OSServiceManager{cmd: rec, osType: "linux"}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := svc.Start(ctx, "mihomo"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if rec.lastCtx != ctx {
		t.Fatal("service command did not receive caller context")
	}
}

func TestOSServiceManagerIsRunningLinux(t *testing.T) {
	rec := &commandRecorder{output: "active"}
	svc := &OSServiceManager{cmd: rec, osType: "linux"}

	running, err := svc.IsRunning(context.Background(), "mihomo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !running {
		t.Error("expected running=true for 'active' output")
	}
	if len(rec.captured) != 1 {
		t.Fatalf("expected 1 command, got %d", len(rec.captured))
	}
	if rec.captured[0].name != "systemctl" {
		t.Errorf("expected systemctl, got %s", rec.captured[0].name)
	}
}

func TestOSServiceManagerIsNotRunningLinux(t *testing.T) {
	rec := &commandRecorder{cmdErr: fmt.Errorf("exit status 1")}
	svc := &OSServiceManager{cmd: rec, osType: "linux"}

	running, err := svc.IsRunning(context.Background(), "mihomo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if running {
		t.Error("expected running=false for 'inactive' output")
	}
}

func TestOSServiceManagerRegisterLinux(t *testing.T) {
	rec := &commandRecorder{}
	svc := &OSServiceManager{cmd: rec, osType: "linux"}

	err := svc.Register(context.Background(), "mihomo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.captured) != 1 {
		t.Fatalf("expected 1 command, got %d", len(rec.captured))
	}
	if rec.captured[0].name != "systemctl" || rec.captured[0].args[0] != "daemon-reload" {
		t.Errorf("expected systemctl daemon-reload, got %v", rec.captured[0])
	}
}

func TestOSServiceManagerEnableAutoStartLinux(t *testing.T) {
	rec := &commandRecorder{}
	svc := &OSServiceManager{cmd: rec, osType: "linux"}

	err := svc.EnableAutoStart(context.Background(), "mihomo", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.captured) != 1 {
		t.Fatalf("expected 1 command, got %d", len(rec.captured))
	}
	if rec.captured[0].name != "systemctl" || rec.captured[0].args[0] != "enable" {
		t.Errorf("expected systemctl enable, got %v", rec.captured[0])
	}
}

func TestOSServiceManagerDisableAutoStartLinux(t *testing.T) {
	rec := &commandRecorder{}
	svc := &OSServiceManager{cmd: rec, osType: "linux"}

	err := svc.DisableAutoStart(context.Background(), "mihomo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.captured) != 1 {
		t.Fatalf("expected 1 command, got %d", len(rec.captured))
	}
	if rec.captured[0].name != "systemctl" || rec.captured[0].args[0] != "disable" {
		t.Errorf("expected systemctl disable, got %v", rec.captured[0])
	}
}

func TestOSServiceManagerAutoStartEnabledLinux(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected bool
	}{
		{"enabled", "enabled", true},
		{"disabled", "disabled", false},
		{"error", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &commandRecorder{output: tt.output}
			svc := &OSServiceManager{cmd: rec, osType: "linux"}
			enabled, err := svc.AutoStartEnabled(context.Background(), "mihomo")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if enabled != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, enabled)
			}
		})
	}
}

func TestOSServiceManagerUnregisterLinux(t *testing.T) {
	rec := &commandRecorder{}
	svc := &OSServiceManager{cmd: rec, osType: "linux"}

	err := svc.Unregister(context.Background(), "mihomo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.captured) != 1 {
		t.Fatalf("expected 1 command, got %d", len(rec.captured))
	}
	if rec.captured[0].name != "systemctl" || rec.captured[0].args[0] != "disable" {
		t.Errorf("expected systemctl disable, got %v", rec.captured[0])
	}
}

func TestOSServiceManagerStartStopRestartLinux(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(*OSServiceManager) error
		expected string
	}{
		{"start", func(s *OSServiceManager) error { return s.Start(context.Background(), "mihomo") }, "start"},
		{"stop", func(s *OSServiceManager) error { return s.Stop(context.Background(), "mihomo") }, "stop"},
		{"restart", func(s *OSServiceManager) error { return s.Restart(context.Background(), "mihomo") }, "restart"},
		{"reload", func(s *OSServiceManager) error { return s.Reload(context.Background(), "mihomo") }, "reload"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &commandRecorder{}
			svc := &OSServiceManager{cmd: rec, osType: "linux"}

			if err := tt.fn(svc); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(rec.captured) != 1 {
				t.Fatalf("expected 1 command, got %d", len(rec.captured))
			}
			if rec.captured[0].name != "systemctl" || rec.captured[0].args[0] != tt.expected {
				t.Errorf("expected systemctl %s, got %v", tt.expected, rec.captured[0])
			}
		})
	}
}

func TestOSServiceManagerIsRunningDarwin(t *testing.T) {
	rec := &commandRecorder{output: "PID 12345 mihomo"}
	svc := &OSServiceManager{cmd: rec, osType: "darwin"}

	running, err := svc.IsRunning(context.Background(), "mihomo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !running {
		t.Error("expected running=true for output containing PID")
	}
	if rec.captured[0].name != "launchctl" {
		t.Errorf("expected launchctl, got %s", rec.captured[0].name)
	}
}

func TestOSServiceManagerRegisterDarwin(t *testing.T) {
	rec := &commandRecorder{output: "success"}
	svc := &OSServiceManager{cmd: rec, osType: "darwin"}

	err := svc.Register(context.Background(), "mihomo", "/path/to/mihomo.plist")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.captured) != 1 {
		t.Fatalf("expected 1 command, got %d", len(rec.captured))
	}
	if rec.captured[0].name != "launchctl" || rec.captured[0].args[0] != "load" {
		t.Errorf("expected launchctl load, got %v", rec.captured[0])
	}
}

func TestOSServiceManagerUnregisterDarwin(t *testing.T) {
	rec := &commandRecorder{output: "success"}
	svc := &OSServiceManager{cmd: rec, osType: "darwin"}

	err := svc.Unregister(context.Background(), "mihomo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rec.captured) != 1 {
		t.Fatalf("expected 1 command, got %d", len(rec.captured))
	}
	if rec.captured[0].name != "launchctl" || rec.captured[0].args[0] != "unload" {
		t.Errorf("expected launchctl unload, got %v", rec.captured[0])
	}
}

func TestOSServiceManagerIsNotRunningDarwin(t *testing.T) {
	rec := &commandRecorder{output: "mihomo\tstopped"}
	svc := &OSServiceManager{cmd: rec, osType: "darwin"}

	running, err := svc.IsRunning(context.Background(), "mihomo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if running {
		t.Error("expected running=false for output without PID")
	}
}

func TestOSServiceManagerUnsupportedOS(t *testing.T) {
	rec := &commandRecorder{}
	svc := &OSServiceManager{cmd: rec, osType: "windows"}

	_, err := svc.IsRunning(context.Background(), "mihomo")
	if err == nil {
		t.Error("expected error for unsupported OS")
	}
}
