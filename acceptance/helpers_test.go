//go:build acceptance

package acceptance

import (
	"bytes"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type RunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

func buildBinary(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "mihomo-manager")
	cmd := exec.Command("go", "build", "-o", tmpPath, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return tmpPath
}

func runMihomo(t *testing.T, binary string, args ...string) RunResult {
	t.Helper()
	cmd := exec.Command("sudo", append([]string{binary}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s %v: %v", binary, args, err)
		}
	}
	return RunResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

func runMihomoNoSudo(t *testing.T, binary string, args ...string) RunResult {
	t.Helper()
	cmd := exec.Command(binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s %v: %v", binary, args, err)
		}
	}
	return RunResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

func runSudo(t *testing.T, name string, args ...string) RunResult {
	t.Helper()
	cmd := exec.Command("sudo", append([]string{name}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run sudo %s %v: %v", name, args, err)
		}
	}
	return RunResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

func runCommand(t *testing.T, name string, args ...string) RunResult {
	t.Helper()
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s %v: %v", name, args, err)
		}
	}
	return RunResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}

func preflightCheck(t *testing.T) {
	t.Helper()

	cmd := exec.Command("sudo", "-n", "true")
	if err := cmd.Run(); err != nil {
		t.Skip("sudo -n true failed: passwordless sudo required")
	}

	cmd = exec.Command("systemctl", "--version")
	if err := cmd.Run(); err != nil {
		t.Skip("systemctl not available: Linux systemd required")
	}

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://github.com")
	if err != nil {
		t.Skip("network unreachable: cannot reach github.com")
	}
	resp.Body.Close()
}

func ensureInstalled(t *testing.T, binary string) {
	t.Helper()
	r := runMihomo(t, binary, "status")
	if r.ExitCode == 0 && strings.Contains(r.Stdout, "running") {
		return
	}
	if r.ExitCode == 1 && strings.Contains(r.Stdout, "stopped") {
		r = runMihomo(t, binary, "start")
		if r.ExitCode != 0 {
			t.Fatalf("start after status=stopped failed: %s %s", r.Stdout, r.Stderr)
		}
		return
	}

	r = runMihomo(t, binary, "install")
	if r.ExitCode != 0 {
		t.Fatalf("install failed: %s %s (exit %d)", r.Stdout, r.Stderr, r.ExitCode)
	}
}

func ensureRunning(t *testing.T, binary string) {
	t.Helper()
	r := runMihomo(t, binary, "status")
	if r.ExitCode == 0 && strings.Contains(r.Stdout, "running") {
		return
	}
	r = runMihomo(t, binary, "start")
	if r.ExitCode != 0 {
		t.Fatalf("start failed: %s %s (exit %d)", r.Stdout, r.Stderr, r.ExitCode)
	}
}

func ensureStopped(t *testing.T, binary string) {
	t.Helper()
	r := runMihomo(t, binary, "status")
	if r.ExitCode == 1 && strings.Contains(r.Stdout, "stopped") {
		return
	}
	r = runMihomo(t, binary, "stop")
	if r.ExitCode != 0 {
		t.Fatalf("stop failed: %s %s (exit %d)", r.Stdout, r.Stderr, r.ExitCode)
	}
}

func ensureUninstalled(t *testing.T, binary string) {
	t.Helper()
	r := runMihomo(t, binary, "status")
	if r.ExitCode == 2 && strings.Contains(r.Stdout, "not installed") {
		return
	}
	r = runMihomo(t, binary, "uninstall")
	if r.ExitCode != 0 {
		t.Logf("uninstall (attempt) returned exit %d: %s %s", r.ExitCode, r.Stdout, r.Stderr)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func assertStdoutContains(t *testing.T, result RunResult, substr string) {
	t.Helper()
	if !strings.Contains(result.Stdout, substr) {
		t.Errorf("expected stdout to contain %q\nstdout:\n%s\nstderr:\n%s", substr, result.Stdout, result.Stderr)
	}
}

func assertExitCode(t *testing.T, result RunResult, expected int) {
	t.Helper()
	if result.ExitCode != expected {
		t.Errorf("expected exit code %d, got %d\nstdout:\n%s\nstderr:\n%s", expected, result.ExitCode, result.Stdout, result.Stderr)
	}
}

func logResult(t *testing.T, result RunResult) {
	t.Helper()
	t.Logf("exit code: %d", result.ExitCode)
	t.Logf("stdout:\n%s", result.Stdout)
	t.Logf("stderr:\n%s", result.Stderr)
}

func getPID(t *testing.T) string {
	t.Helper()
	r := runSudo(t, "systemctl", "show", "--property=MainPID", "--value", "mihomo")
	if r.ExitCode != 0 {
		t.Fatalf("failed to get PID: %s %s", r.Stdout, r.Stderr)
	}
	pid := strings.TrimSpace(r.Stdout)
	if pid == "0" {
		return ""
	}
	return pid
}


func writeFile(t *testing.T, path, content string) {
	t.Helper()
	cmd := exec.Command("sudo", "tee", path)
	cmd.Stdin = strings.NewReader(content)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("writeFile %s failed: %v\n%s", path, err, out)
	}
}

func removeFile(t *testing.T, path string) {
	t.Helper()
	cmd := exec.Command("sudo", "rm", "-rf", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("removeFile %s failed: %v\n%s", path, err, out)
	}
}

func chmodFile(t *testing.T, path, mode string) {
	t.Helper()
	cmd := exec.Command("sudo", "chmod", mode, path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("chmodFile %s %s failed: %v\n%s", path, mode, err, out)
	}
}

func readFileSudo(t *testing.T, path string) string {
	t.Helper()
	cmd := exec.Command("sudo", "cat", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("readFileSudo %s failed: %v\n%s", path, err, out)
	}
	return string(out)
}

func listDir(t *testing.T, path string) []string {
	t.Helper()
	cmd := exec.Command("sudo", "ls", "-1", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	return lines
}

func dirExists(t *testing.T, path string) bool {
	t.Helper()
	cmd := exec.Command("sudo", "test", "-d", path)
	return cmd.Run() == nil
}

func ensureBackToRunning(t *testing.T, binary string) {
	t.Helper()
	r := runMihomo(t, binary, "status")
	if r.ExitCode == 0 && strings.Contains(r.Stdout, "running") {
		return
	}
	if r.ExitCode == 1 && strings.Contains(r.Stdout, "stopped") {
		r = runMihomo(t, binary, "start")
		if r.ExitCode != 0 {
			t.Fatalf("ensureRunning: start failed: %s %s", r.Stdout, r.Stderr)
		}
		return
	}

	t.Logf("ensureRunning: status unexpected (%d), reinstalling", r.ExitCode)
	ensureUninstalled(t, binary)
	r = runMihomo(t, binary, "install")
	if r.ExitCode != 0 {
		t.Fatalf("ensureRunning: reinstall failed: %s %s", r.Stdout, r.Stderr)
	}
}

var installPhases = []string{"[fetch]", "[deploy]", "[bootstrap]", "[register]", "[start]"}
var uninstallPhases = []string{"[stop]", "[deregister]", "[clean]"}
