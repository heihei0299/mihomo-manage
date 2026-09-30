package manager

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type validationCommandRunner struct {
	name              string
	args              []string
	ctx               context.Context
	out               string
	err               error
	started           chan struct{}
	waitForCancel     bool
	startedOnceClosed bool
}

func (r *validationCommandRunner) RunCommand(ctx context.Context, name string, args ...string) (string, error) {
	r.ctx = ctx
	r.name = name
	r.args = append([]string(nil), args...)
	if r.started != nil && !r.startedOnceClosed {
		close(r.started)
		r.startedOnceClosed = true
	}
	if r.waitForCancel {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return r.out, r.err
}

func (r *validationCommandRunner) RunCommandIgnoreExit(context.Context, string, ...string) (string, error) {
	return r.out, r.err
}

func TestConfigValidatorUsesCommandRunnerForStagedConfig(t *testing.T) {
	runner := &validationCommandRunner{}
	validator := NewConfigValidator(runner)
	ctx := context.WithValue(context.Background(), struct{}{}, "marker")

	if err := validator.Validate(ctx, "/tmp/config-staging/config.yaml"); err != nil {
		t.Fatalf("Validate failed: %v", err)
	}
	if runner.name != binaryPath {
		t.Fatalf("command = %q, want %q", runner.name, binaryPath)
	}
	wantArgs := []string{"-t", "-d", configDir, "-f", "/tmp/config-staging/config.yaml"}
	if strings.Join(runner.args, "\x00") != strings.Join(wantArgs, "\x00") {
		t.Fatalf("args = %v, want %v", runner.args, wantArgs)
	}
	if runner.ctx == nil || runner.ctx.Value(struct{}{}) != "marker" {
		t.Fatal("validator did not pass caller context to command runner")
	}
}

func TestConfigValidatorPreservesCommandOutputOnFailure(t *testing.T) {
	commandErr := errors.New("exit status 1")
	runner := &validationCommandRunner{out: "parse error at line 4", err: commandErr}
	validator := NewConfigValidator(runner)

	err := validator.Validate(context.Background(), "/tmp/config.yaml")
	if !errors.Is(err, commandErr) {
		t.Fatalf("Validate error = %v, want command error", err)
	}
	if !strings.Contains(err.Error(), "parse error at line 4") {
		t.Fatalf("Validate error = %v, want command output", err)
	}
}

func TestConfigApplyStatusIncludesValidationCommandDiagnostic(t *testing.T) {
	runner := &validationCommandRunner{
		out: "parse error at line 4",
		err: errors.New("exit status 1"),
	}
	m := newTestConfigManager(localApplyTestFileSystem(), &fakeReleaseSource{}, NewConfigValidator(runner), noopReload)

	if err := m.UpdateConfig(context.Background()); err == nil {
		t.Fatal("UpdateConfig should report validation failure")
	}
	status, err := m.LastConfigApply(context.Background())
	if err != nil {
		t.Fatalf("LastConfigApply failed: %v", err)
	}
	if status.State != ConfigValidationFailed || !strings.Contains(status.ErrorSummary, "parse error at line 4") {
		t.Fatalf("status = %+v, want validation diagnostic", status)
	}
}

func TestConfigValidatorReturnsCancellationUnwrapped(t *testing.T) {
	runner := &validationCommandRunner{started: make(chan struct{}), waitForCancel: true}
	validator := NewConfigValidator(runner)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- validator.Validate(ctx, "/tmp/config.yaml") }()
	<-runner.started
	cancel()

	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("Validate error = %v, want context cancellation", err)
	}
	if runner.ctx == nil || !errors.Is(runner.ctx.Err(), context.Canceled) {
		t.Fatal("command runner did not receive the canceled context")
	}
}

// Translate only the production executable/home at the OS boundary. The
// validator still chooses its own arguments and the real core parses them.
type realValidationRunner struct{ binary, home string }

func (r realValidationRunner) RunCommand(ctx context.Context, name string, args ...string) (string, error) {
	if name != binaryPath {
		return "", errors.New("unexpected executable")
	}
	args = append([]string(nil), args...)
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-d" && args[i+1] == configDir {
			args[i+1] = r.home
		}
	}
	out, err := exec.CommandContext(ctx, r.binary, args...).CombinedOutput()
	return string(out), err
}
func (r realValidationRunner) RunCommandIgnoreExit(ctx context.Context, name string, args ...string) (string, error) {
	return r.RunCommand(ctx, name, args...)
}

func TestConfigValidatorRealCoreUsesRuntimeHomeForStaging(t *testing.T) {
	binary := os.Getenv("MIHOMO_TEST_BINARY")
	if binary == "" {
		t.Skip("set MIHOMO_TEST_BINARY to run real-core validation")
	}
	home := t.TempDir()
	staged := filepath.Join(home, ".mihomo-config-staging-test")
	if err := os.MkdirAll(staged, 0700); err != nil {
		t.Fatal(err)
	}
	rules := filepath.Join(home, "local.yaml")
	if err := os.WriteFile(rules, []byte("payload:\n  - example.com\n"), 0600); err != nil {
		t.Fatal(err)
	}
	config := "mode: rule\nrule-providers:\n  local:\n    type: file\n    behavior: domain\n    path: " + rules + "\nrules:\n  - RULE-SET,local,DIRECT\n  - MATCH,DIRECT\n"
	candidate := filepath.Join(staged, "config.yaml")
	if err := os.WriteFile(candidate, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	validator := NewConfigValidator(realValidationRunner{binary: binary, home: home})
	if err := validator.Validate(context.Background(), candidate); err != nil {
		t.Fatal(err)
	}
}
