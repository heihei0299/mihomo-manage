//go:build acceptance

package acceptance

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	binaryPath          = "/opt/mihomo/bin/mihomo"
	configDir           = "/opt/mihomo/etc"
	configTemplatePath  = "/opt/mihomo/etc/config-template.yaml"
	configYAML          = "/opt/mihomo/etc/config.yaml"
	backupsDir          = "/opt/mihomo-manager/backups"
	subscriptionURLFile = "/opt/mihomo-manager/state/subscription-url.txt"
	subscriptionDataFile = "/opt/mihomo-manager/state/subscription-data.txt"
)

var testURL = "https://raw.githubusercontent.com/MetaCubeX/mihomo-dashboard/refs/heads/gh-pages/index.html"

func buildBinaryAndPreflight(t *testing.T) string {
	t.Helper()
	preflightCheck(t)
	return buildBinary(t)
}

func TestAcceptanceInstall(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureUninstalled(t, binary)

	r := runMihomo(t, binary, "install")
	assertExitCode(t, r, 0)

	for _, phase := range installPhases {
		assertStdoutContains(t, r, phase)
	}
	assertStdoutContains(t, r, "✓")

	if !fileExists(binaryPath) {
		t.Error("expected /opt/mihomo/bin/mihomo to exist after install")
	}
	if !fileExists(configTemplatePath) {
		t.Error("expected /opt/mihomo/etc/config-template.yaml to exist after install")
	}
	tmpl := readFileSudo(t, configTemplatePath)
	if !strings.Contains(tmpl, "{{subscription}}") {
		t.Error("expected config-template.yaml to contain {{subscription}}")
	}
	if !fileExists(configYAML) {
		t.Error("expected /opt/mihomo/etc/config.yaml to exist after install")
	}
	cfg := readFileSudo(t, configYAML)
	if !strings.Contains(cfg, "port: 7890") {
		t.Errorf("expected config.yaml to contain 'port: 7890', got:\n%s", cfg)
	}

	svc := runSudo(t, "systemctl", "is-active", "mihomo")
	if strings.TrimSpace(svc.Stdout) != "active" {
		t.Errorf("expected systemctl is-active mihomo → active, got %q", svc.Stdout)
	}
}

func TestAcceptanceSystemdService(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)

	active := runSudo(t, "systemctl", "is-active", "mihomo")
	if strings.TrimSpace(active.Stdout) != "active" {
		t.Errorf("expected systemctl is-active mihomo → active, got %q", active.Stdout)
	}

	enabled := runSudo(t, "systemctl", "is-enabled", "mihomo")
	if strings.TrimSpace(enabled.Stdout) != "enabled" {
		t.Errorf("expected systemctl is-enabled mihomo → enabled, got %q", enabled.Stdout)
	}

	status := runSudo(t, "systemctl", "status", "mihomo", "--no-pager")
	assertStdoutContains(t, status, "active (running)")
}

func TestAcceptanceStatus(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)
	ensureRunning(t, binary)

	r := runMihomo(t, binary, "status")
	assertExitCode(t, r, 0)
	if !strings.Contains(r.Stdout, "mihomo: running") {
		t.Errorf("expected 'mihomo: running' in status, got:\n%s", r.Stdout)
	}
	if !strings.Contains(r.Stdout, "version:") {
		t.Errorf("expected version in status output, got:\n%s", r.Stdout)
	}

	ensureStopped(t, binary)
	r = runMihomo(t, binary, "status")
	if r.ExitCode == 0 {
		t.Error("expected non-zero exit code for stopped status")
	}
	if !strings.Contains(r.Stdout, "mihomo: stopped") {
		t.Errorf("expected 'mihomo: stopped' in status, got:\n%s", r.Stdout)
	}
	ensureRunning(t, binary)

	ensureUninstalled(t, binary)
	r = runMihomo(t, binary, "status")
	if r.ExitCode != 2 {
		t.Errorf("expected exit code 2 for not-installed status, got %d", r.ExitCode)
	}
	if !strings.Contains(r.Stdout, "mihomo: not installed") {
		t.Errorf("expected 'mihomo: not installed' in status, got:\n%s", r.Stdout)
	}
	ensureInstalled(t, binary)
}

func TestAcceptanceStop(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)
	ensureRunning(t, binary)

	r := runMihomo(t, binary, "stop")
	assertExitCode(t, r, 0)

	active := runSudo(t, "systemctl", "is-active", "mihomo")
	if strings.TrimSpace(active.Stdout) != "inactive" {
		t.Errorf("expected systemctl is-active → inactive, got %q", active.Stdout)
	}

	pgrep := runSudo(t, "pgrep", "-x", "mihomo")
	if pgrep.ExitCode == 0 {
		t.Error("expected pgrep -x mihomo to exit non-zero after stop")
	}

	status := runMihomo(t, binary, "status")
	if status.ExitCode == 0 {
		t.Error("expected status to exit non-zero after stop")
	}

	ensureRunning(t, binary)
}

func TestAcceptanceStart(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)
	ensureStopped(t, binary)

	r := runMihomo(t, binary, "start")
	assertExitCode(t, r, 0)

	active := runSudo(t, "systemctl", "is-active", "mihomo")
	if strings.TrimSpace(active.Stdout) != "active" {
		t.Errorf("expected systemctl is-active → active, got %q", active.Stdout)
	}

	pgrep := runSudo(t, "pgrep", "-x", "mihomo")
	if pgrep.ExitCode != 0 {
		t.Error("expected pgrep -x mihomo to exit 0 after start")
	}

	status := runMihomo(t, binary, "status")
	assertExitCode(t, status, 0)
}

func TestAcceptanceReload(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)
	ensureRunning(t, binary)

	pidBefore := getPID(t)
	if pidBefore == "" {
		t.Fatal("could not get PID before reload")
	}

	r := runMihomo(t, binary, "reload")
	assertExitCode(t, r, 0)

	time.Sleep(500 * time.Millisecond)

	pidAfter := getPID(t)
	if pidAfter == "" {
		t.Fatal("could not get PID after reload")
	}

	if pidBefore != pidAfter {
		t.Errorf("expected PID unchanged after reload, before=%s after=%s", pidBefore, pidAfter)
	}

	active := runSudo(t, "systemctl", "is-active", "mihomo")
	if strings.TrimSpace(active.Stdout) != "active" {
		t.Errorf("expected systemctl is-active → active after reload, got %q", active.Stdout)
	}
}

func TestAcceptanceRestart(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)
	ensureRunning(t, binary)

	pidBefore := getPID(t)
	if pidBefore == "" {
		t.Fatal("could not get PID before restart")
	}

	r := runMihomo(t, binary, "restart")
	assertExitCode(t, r, 0)

	time.Sleep(500 * time.Millisecond)

	pidAfter := getPID(t)
	if pidAfter == "" {
		t.Fatal("could not get PID after restart")
	}

	if pidBefore == pidAfter {
		t.Errorf("expected PID to change after restart, before=%s after=%s", pidBefore, pidAfter)
	}

	active := runSudo(t, "systemctl", "is-active", "mihomo")
	if strings.TrimSpace(active.Stdout) != "active" {
		t.Errorf("expected systemctl is-active → active after restart, got %q", active.Stdout)
	}
}

func TestAcceptanceSubscriptionSet(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)

	r := runMihomo(t, binary, "subscription", "set", testURL)
	assertExitCode(t, r, 0)

	if !fileExists(subscriptionURLFile) {
		t.Error("expected subscription-url.txt to exist")
	}
	content := readFileSudo(t, subscriptionURLFile)
	if strings.TrimSpace(content) != testURL {
		t.Errorf("expected subscription-url.txt to contain URL, got %q", content)
	}

	localData := "proxies:\n  - name: test\n    type: ss\n    server: example.com\n    port: 8388"
	r = runMihomo(t, binary, "subscription", "set", localData)
	assertExitCode(t, r, 0)

	if !fileExists(subscriptionDataFile) {
		t.Error("expected subscription-data.txt to exist after local set")
	}
	data := readFileSudo(t, subscriptionDataFile)
	if !strings.Contains(data, "proxies:") {
		t.Errorf("expected subscription-data.txt to contain local data, got:\n%s", data)
	}

	if fileExists(subscriptionURLFile) {
		t.Error("expected subscription-url.txt to NOT exist after local data set")
	}

	r = runMihomo(t, binary, "subscription", "set")
	if r.ExitCode == 0 {
		t.Error("expected non-zero exit code for subscription set with no args")
	}

	r = runMihomo(t, binary, "subscription", "set", "https://invalid.example.com/sub")
	assertExitCode(t, r, 0)
	r = runMihomo(t, binary, "subscription", "update")
	if r.ExitCode == 0 {
		t.Error("expected non-zero exit code when subscription URL unreachable")
	}
}

func TestAcceptanceSubscriptionUpdate(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)

	runMihomo(t, binary, "subscription", "set", testURL)

	r := runMihomo(t, binary, "subscription", "update")
	assertExitCode(t, r, 0)

	config := readFileSudo(t, configYAML)
	if !strings.Contains(config, "proxies") {
		t.Errorf("expected config.yaml to contain proxies after update, got:\n%s", config)
	}

	backups := listDir(t, configDir)
	found := false
	for _, f := range backups {
		if strings.HasPrefix(f, "config.yaml.bak.") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a config.yaml.bak.<timestamp> backup in /opt/mihomo/etc/")
	}

	active := runSudo(t, "systemctl", "is-active", "mihomo")
	if strings.TrimSpace(active.Stdout) != "active" {
		t.Errorf("expected service active after update, got %q", active.Stdout)
	}
}

func TestAcceptanceConfigPreview(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)

	runMihomo(t, binary, "subscription", "set", "proxies:\n  - name: test\n    type: ss\n    server: example.com\n    port: 8388")
	runMihomo(t, binary, "subscription", "update")
	ensureRunning(t, binary)

	r := runMihomo(t, binary, "config", "preview")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "port:")
	assertStdoutContains(t, r, "proxies:")
	assertStdoutContains(t, r, "proxy-groups:")
	assertStdoutContains(t, r, "rules:")
}

func TestAcceptanceVersions(t *testing.T) {
	binary := buildBinaryAndPreflight(t)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.github.com")
	if err != nil {
		t.Skip("GitHub API unreachable")
	}
	resp.Body.Close()

	r := runMihomo(t, binary, "versions")
	assertExitCode(t, r, 0)

	lines := strings.Split(strings.TrimSpace(r.Stdout), "\n")
	var versionLines []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			versionLines = append(versionLines, strings.TrimSpace(l))
		}
	}
	if len(versionLines) < 1 {
		t.Fatalf("expected at least 1 version line, got 0")
	}
	if len(versionLines) > 5 {
		t.Fatalf("expected ≤5 version lines, got %d", len(versionLines))
	}
	for _, line := range versionLines {
		if len(line) < 3 || line[0] != 'v' || line[1] < '0' || line[1] > '9' {
			t.Errorf("expected version line like v<digits>..., got %q", line)
		}
	}
}

func TestAcceptanceUpgrade(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)
	ensureRunning(t, binary)

	currentVersion := getCurrentVersion(t, binary)
	if currentVersion == "" {
		t.Fatal("could not determine current version")
	}

	r := runMihomo(t, binary, "upgrade", currentVersion)
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "[fetch]")
	assertStdoutContains(t, r, "[deploy]")
	assertStdoutContains(t, r, "[start]")

	if !fileExists(backupsDir + "/mihomo.bak") {
		t.Error("expected /opt/mihomo-manager/backups/mihomo.bak to exist after upgrade")
	}

	active := runSudo(t, "systemctl", "is-active", "mihomo")
	if strings.TrimSpace(active.Stdout) != "active" {
		t.Errorf("expected service active after upgrade, got %q", active.Stdout)
	}
}

func getCurrentVersion(t *testing.T, binary string) string {
	t.Helper()
	r := runMihomo(t, binary, "status")
	for _, line := range strings.Split(r.Stdout, "\n") {
		if strings.Contains(line, "version:") {
			parts := strings.Split(line, "version:")
			if len(parts) > 1 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func TestAcceptanceUpgradeRollback(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)
	ensureRunning(t, binary)

	currentVersion := getCurrentVersion(t, binary)
	if currentVersion == "" {
		t.Fatal("could not determine current version")
	}

	backupBinary := "/tmp/mihomo.bak.test"
	runSudo(t, "cp", binaryPath, backupBinary)

	fakeScript := "#!/bin/sh\nexit 1\n"
	writeFile(t, binaryPath, fakeScript)
	chmodFile(t, binaryPath, "0755")

	t.Cleanup(func() {
		runSudo(t, "cp", backupBinary, binaryPath)
		removeFile(t, backupBinary)
		ensureBackToRunning(t, binary)
	})

	r := runMihomo(t, binary, "upgrade", currentVersion)
	if r.ExitCode != 0 {
		t.Logf("upgrade returned exit %d, checking rollback state", r.ExitCode)
	}

	versionAfter := getCurrentVersion(t, binary)
	t.Logf("version before: %s, after: %s", currentVersion, versionAfter)

	active := runSudo(t, "systemctl", "is-active", "mihomo")
	if strings.TrimSpace(active.Stdout) != "active" {
		t.Errorf("expected service active after upgrade/rollback, got %q", active.Stdout)
	}
}

func TestAcceptanceSchedule(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)

	r := runMihomo(t, binary, "subscription", "schedule", "--interval", "6h")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "schedule set to every 6h0m0s")

	r = runMihomo(t, binary, "subscription", "schedule")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "schedule: every 6h0m0s")

	r = runMihomo(t, binary, "subscription", "schedule", "--off")
	assertExitCode(t, r, 0)

	time.Sleep(100 * time.Millisecond)

	r = runMihomo(t, binary, "subscription", "schedule")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "schedule: off")

	r = runMihomo(t, binary, "subscription", "schedule", "--interval", "30s")
	if r.ExitCode == 0 {
		t.Error("expected non-zero exit code for interval < 1h")
	}
}

func TestAcceptanceLogs(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)
	ensureRunning(t, binary)

	r := runMihomo(t, binary, "logs", "--tail=5")
	assertExitCode(t, r, 0)

	r = runMihomo(t, binary, "logs", "--tail=100")
	assertExitCode(t, r, 0)

	r = runMihomo(t, binary, "logs")
	assertExitCode(t, r, 0)
}

func TestAcceptanceQuietMode(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)
	ensureRunning(t, binary)

	r := runMihomoNoSudo(t, binary, "--quiet", "status")
	assertExitCode(t, r, 0)
	if strings.TrimSpace(r.Stdout) != "" {
		t.Errorf("expected empty stdout in --quiet mode (running), got:\n%s", r.Stdout)
	}

	r = runMihomoNoSudo(t, binary, "-q", "status")
	assertExitCode(t, r, 0)
	if strings.TrimSpace(r.Stdout) != "" {
		t.Errorf("expected empty stdout in -q mode (running), got:\n%s", r.Stdout)
	}

	ensureUninstalled(t, binary)
	r = runMihomoNoSudo(t, binary, "--quiet", "status")
	if r.ExitCode != 2 {
		t.Errorf("expected exit code 2 for not-installed --quiet, got %d", r.ExitCode)
	}
	if strings.TrimSpace(r.Stdout) != "" {
		t.Errorf("expected empty stdout in --quiet mode (not installed), got:\n%s", r.Stdout)
	}

	r = runMihomo(t, binary, "stop", "--quiet")
	if strings.TrimSpace(r.Stderr) != "" {
		t.Logf("stderr with error: %s", r.Stderr)
	}
	ensureInstalled(t, binary)
}

func TestAcceptanceUninstall(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)
	ensureRunning(t, binary)

	r := runMihomo(t, binary, "uninstall")
	assertExitCode(t, r, 0)
	assertStdoutContains(t, r, "[stop]")
	assertStdoutContains(t, r, "[deregister]")
	assertStdoutContains(t, r, "[clean]")

	if fileExists(binaryPath) {
		t.Error("expected /opt/mihomo/bin/mihomo to not exist after uninstall")
	}
	if fileExists(configDir) {
		t.Error("expected /opt/mihomo/etc to not exist after uninstall")
	}
	if fileExists("/opt/mihomo-manager") {
		t.Error("expected /opt/mihomo-manager/ to not exist after uninstall")
	}

	listUnits := runSudo(t, "systemctl", "list-units", "--no-pager")
	if strings.Contains(listUnits.Stdout, "mihomo") {
		t.Error("expected no systemd units matching mihomo after uninstall")
	}

	status := runMihomo(t, binary, "status")
	if status.ExitCode != 2 {
		t.Errorf("expected exit code 2 for status after uninstall, got %d", status.ExitCode)
	}
	if !strings.Contains(status.Stdout, "mihomo: not installed") {
		t.Errorf("expected 'mihomo: not installed' after uninstall, got:\n%s", status.Stdout)
	}

	r2 := runMihomo(t, binary, "uninstall")
	if r2.ExitCode == 0 {
		t.Error("expected uninstall to fail when not installed")
	}
}

func TestAcceptanceUninstallKeepBackup(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureUninstalled(t, binary)
	r := runMihomo(t, binary, "install")
	assertExitCode(t, r, 0)
	ensureRunning(t, binary)

	runMihomo(t, binary, "subscription", "set", "proxies:\n  - name: test\n    type: ss")
	runMihomo(t, binary, "subscription", "update")
	ensureRunning(t, binary)

	r = runMihomo(t, binary, "uninstall", "--keep-backup")
	assertExitCode(t, r, 0)

	if fileExists(binaryPath) {
		t.Error("expected /opt/mihomo/bin/mihomo to not exist after uninstall --keep-backup")
	}

	backupDirs := listDir(t, "/opt")
	foundBackup := false
	for _, entry := range backupDirs {
		if strings.HasPrefix(entry, "mihomo.bak.") {
			foundBackup = true
			r := runSudo(t, "test", "-f", "/opt/"+entry+"/etc/config.yaml")
			if r.ExitCode == 0 {
				t.Logf("config preserved in backup: /opt/%s/etc/config.yaml", entry)
			}
			break
		}
	}
	if !foundBackup {
		t.Log("no /opt/mihomo.bak.* directory found (--keep-backup renames /opt/mihomo)")
	}

	listUnits := runSudo(t, "systemctl", "list-units", "--no-pager")
	if strings.Contains(listUnits.Stdout, "mihomo") {
		t.Error("expected no systemd units matching mihomo after uninstall")
	}
}

func TestAcceptanceTunInterface(t *testing.T) {
	binary := buildBinaryAndPreflight(t)
	ensureInstalled(t, binary)
	ensureRunning(t, binary)

	config := readFileSudo(t, configYAML)
	if !strings.Contains(config, "tun:") {
		t.Skip("TUN not configured in config.yaml, skipping")
	}

	ipLink := runSudo(t, "ip", "link", "show", "meta")
	if ipLink.ExitCode != 0 {
		t.Skip("no meta TUN interface found")
	}

	if !strings.Contains(ipLink.Stdout, "UP") {
		t.Error("expected meta interface to have UP flag")
	}
	if !strings.Contains(ipLink.Stdout, "LOWER_UP") {
		t.Error("expected meta interface to have LOWER_UP flag")
	}
}
