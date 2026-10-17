package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/heihei0299/mihomo-manage/internal/manager"
)

type tuiMockControl struct {
	startCalled   bool
	restartCalled bool
	startErr      error
	restartErr    error
}

func (m *tuiMockControl) Status(ctx context.Context) (*manager.Status, error) {
	return &manager.Status{Installed: true, InstanceState: manager.Running}, nil
}

func (m *tuiMockControl) Start(ctx context.Context) error {
	m.startCalled = true
	return m.startErr
}

func (m *tuiMockControl) Stop(ctx context.Context) error { return nil }

func (m *tuiMockControl) Restart(ctx context.Context) error {
	m.restartCalled = true
	return m.restartErr
}

func (m *tuiMockControl) Reload(ctx context.Context) error { return nil }

func (m *tuiMockControl) SetAutoStart(ctx context.Context, enabled bool) error { return nil }

type tuiMockLifecycle struct{}

func (m *tuiMockLifecycle) Install(ctx context.Context, version string, autoStart bool, onProgress manager.ProgressCallback) error {
	return nil
}

func (m *tuiMockLifecycle) InstallFromLocal(ctx context.Context, localPath string, autoStart bool, onProgress manager.ProgressCallback) error {
	return nil
}

func (m *tuiMockLifecycle) Uninstall(ctx context.Context, keepBackup bool, onProgress manager.ProgressCallback) error {
	return nil
}

func (m *tuiMockLifecycle) Upgrade(ctx context.Context, version string, onProgress manager.ProgressCallback) error {
	return nil
}

func (m *tuiMockLifecycle) ListVersions(ctx context.Context) ([]manager.VersionInfo, error) {
	return nil, nil
}

type tuiMockConfig struct {
	updateCalled bool
}

func (m *tuiMockConfig) SetSubscriptionSource(ctx context.Context, url string) error { return nil }

func (m *tuiMockConfig) SetRoutingRules(ctx context.Context, rules string) error { return nil }

func (m *tuiMockConfig) PreviewConfig(ctx context.Context) (string, error) { return "", nil }

func (m *tuiMockConfig) UpdateConfig(ctx context.Context) error {
	m.updateCalled = true
	return nil
}

func (m *tuiMockConfig) ValidateConfig(ctx context.Context) error { return nil }

func (m *tuiMockConfig) LastConfigApply(ctx context.Context) (manager.ConfigApplyStatus, error) {
	return manager.ConfigApplyStatus{State: manager.ConfigUnknown}, nil
}

func (m *tuiMockConfig) AdoptConfig(ctx context.Context, force bool) (manager.AdoptReport, error) {
	return manager.AdoptReport{NoChanges: true}, nil
}

// runActionCmd executes an action command synchronously and returns its error.
func runActionCmd(ctrl manager.ServiceControl, lifecycle manager.LifecycleManager, a action) error {
	cmd := execActionCmd(ctrl, lifecycle, a, nil, "", false)
	msg := cmd()
	done, ok := msg.(actionDoneMsg)
	if !ok {
		return errors.New("expected actionDoneMsg")
	}
	return done.err
}

func TestTUIStatusShowsConfigValidationDiagnostic(t *testing.T) {
	m := model{
		status: &manager.Status{Installed: true, InstanceState: manager.Running},
		configStatus: manager.ConfigApplyStatus{
			State:        manager.ConfigValidationFailed,
			ErrorSummary: "parse error at line 4",
		},
	}

	if got := m.statusView(); !strings.Contains(got, "parse error at line 4") {
		t.Fatalf("status view = %q, want validation diagnostic", got)
	}
}

func TestTUIStatusShowsConfigApplyFailure(t *testing.T) {
	m := model{
		status: &manager.Status{Installed: true, InstanceState: manager.Running},
		configStatus: manager.ConfigApplyStatus{
			State:        manager.ConfigApplyFailed,
			ErrorSummary: "writing staged config failed",
		},
	}

	if got := m.statusView(); !strings.Contains(got, "config: apply-failed") {
		t.Fatalf("status view = %q, want apply-failed status", got)
	}
}

func TestTUIVersionSelectionShowsLookupFailure(t *testing.T) {
	m := model{mode: modeChooseVersion}
	updated, _ := m.Update(versionsMsg{err: errors.New("release lookup failed")})

	view := updated.(model).versionChoiceView()
	if !strings.Contains(view, "release lookup failed") {
		t.Fatalf("version choice view = %q, want lookup failure", view)
	}
}

func TestTUIVersionSelectionShowsEmptyResult(t *testing.T) {
	m := model{mode: modeChooseVersion}
	updated, _ := m.Update(versionsMsg{versions: []manager.VersionInfo{}})

	view := updated.(model).versionChoiceView()
	if !strings.Contains(view, "No releases") {
		t.Fatalf("version choice view = %q, want empty-result message", view)
	}
}

func TestTUIScheduleStatusShowsLegacy(t *testing.T) {
	m := model{
		status:      &manager.Status{Installed: true, InstanceState: manager.Running},
		scheduleErr: fmt.Errorf("schedule lookup: %w", manager.LegacyScheduleError{Interval: time.Hour}),
	}

	if got := m.statusView(); !strings.Contains(got, "schedule: legacy") {
		t.Fatalf("status view = %q, want legacy schedule status", got)
	}
}

func TestTUIConfigViewShowsSubscriptionEditError(t *testing.T) {
	m := model{configTab: configTabSubscription, execResult: "failed", actionErr: errors.New("config update failed")}
	if got := m.configView(); !strings.Contains(got, "config update failed") {
		t.Fatalf("config view = %q, want edit error", got)
	}
}

func TestTUISubscriptionConfigOffersEditor(t *testing.T) {
	m := model{configTab: configTabSubscription}
	if !strings.Contains(m.configView(), "e) Edit") {
		t.Fatalf("subscription config view should offer an editor: %s", m.configView())
	}
}

func TestEditorCommandPreservesArguments(t *testing.T) {
	cmd, err := editorCommand("code --wait", "/tmp/subscription")
	if err != nil {
		t.Fatalf("editorCommand failed: %v", err)
	}
	if filepath.Base(cmd.Path) != "code" {
		t.Fatalf("editor path = %q, want code", cmd.Path)
	}
	want := []string{"code", "--wait", "/tmp/subscription"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("editor args = %v, want %v", cmd.Args, want)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Fatalf("editor args = %v, want %v", cmd.Args, want)
		}
	}
}

func TestTUIStartPropagatesServiceControlError(t *testing.T) {
	want := errors.New("start failed")
	ctrl := &tuiMockControl{startErr: want}

	if err := runActionCmd(ctrl, &tuiMockLifecycle{}, actStart); !errors.Is(err, want) {
		t.Fatalf("start error = %v, want %v", err, want)
	}
}

func TestProgressReaderConsumesMultipleMessages(t *testing.T) {
	ch := make(chan progressMsg, 2)
	ch <- progressMsg{phase: manager.PhaseUpgradeCheck, message: "first"}
	ch <- progressMsg{phase: manager.PhaseUpgradeFetch, message: "second"}
	close(ch)

	m := model{}
	msg := progressReaderCmd(ch)()
	updated, next := m.Update(msg)
	if updated.(model).phaseMsg != "first" {
		t.Fatalf("first progress = %q, want first", updated.(model).phaseMsg)
	}
	if next == nil {
		t.Fatal("first progress should schedule the next channel read")
	}

	msg = next()
	updated, next = updated.(model).Update(msg)
	if updated.(model).phaseMsg != "second" {
		t.Fatalf("second progress = %q, want second", updated.(model).phaseMsg)
	}
	if next == nil {
		t.Fatal("second progress should consume the closed-channel read")
	}
	if msg = next(); msg != nil {
		t.Fatalf("closed progress channel returned %#v, want nil", msg)
	}
}

type tuiProgressLifecycle struct {
	*tuiMockLifecycle
	err error
}

func (m *tuiProgressLifecycle) Upgrade(ctx context.Context, version string, onProgress manager.ProgressCallback) error {
	onProgress(manager.ProgressEvent{Phase: manager.PhaseUpgradeFetch, Message: "fetching"})
	return m.err
}

func TestProgressReaderKeepsActionDoneError(t *testing.T) {
	want := errors.New("upgrade failed")
	progressCh := make(chan progressMsg, 2)
	cmd := execActionCmdWithContext(
		context.Background(),
		&tuiMockControl{},
		&tuiProgressLifecycle{tuiMockLifecycle: &tuiMockLifecycle{}, err: want},
		actUpgrade,
		progressCh,
		"v1.2.3",
		false,
	)

	msg := cmd()
	done, ok := msg.(actionDoneMsg)
	if !ok || !errors.Is(done.err, want) {
		t.Fatalf("action result = %#v, want actionDone error %v", msg, want)
	}
	if progress, ok := <-progressCh; !ok || progress.message != "fetching" {
		t.Fatalf("progress = %#v, want fetching progress", progress)
	}
	if _, ok := <-progressCh; ok {
		t.Fatal("progress channel should close after actionDone is produced")
	}
}

func TestTUIUpgradeCompletionRefreshesStatus(t *testing.T) {
	m := model{
		executing: actUpgrade,
		status:    &manager.Status{Installed: true, InstanceState: manager.Stopped},
		control:   &tuiMockControl{},
		config:    &tuiMockConfig{},
	}

	updated, refresh := m.Update(actionDoneMsg{action: actUpgrade})
	if updated.(model).executing != actNone {
		t.Fatal("completed upgrade should leave executing mode")
	}
	status, ok := refresh().(statusMsg)
	if !ok || status.status == nil || status.status.InstanceState != manager.Running {
		t.Fatalf("refresh result = %#v, want running status", status)
	}
}

func TestTUIUpgradeFailureStillRefreshesStatus(t *testing.T) {
	m := model{
		executing: actUpgrade,
		status:    &manager.Status{Installed: true, InstanceState: manager.Running},
		control:   &tuiMockControl{},
		config:    &tuiMockConfig{},
	}

	updated, refresh := m.Update(actionDoneMsg{action: actUpgrade, err: errors.New("rollback failed")})
	if updated.(model).execResult != "failed" {
		t.Fatal("failed upgrade should be displayed as failed")
	}
	status, ok := refresh().(statusMsg)
	if !ok || status.status == nil {
		t.Fatalf("refresh result = %#v, want status refresh", status)
	}
	refreshed, _ := updated.(model).Update(status)
	if !strings.Contains(refreshed.(model).statusView(), "rollback failed") {
		t.Fatal("status refresh should retain the upgrade failure diagnostic")
	}
}

func TestTUIStartCallsServiceControl(t *testing.T) {
	ctrl := &tuiMockControl{}

	if err := runActionCmd(ctrl, &tuiMockLifecycle{}, actStart); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctrl.startCalled {
		t.Error("Start should be called")
	}
}

func TestTUIRestartPropagatesServiceControlError(t *testing.T) {
	want := errors.New("restart failed")
	ctrl := &tuiMockControl{restartErr: want}

	if err := runActionCmd(ctrl, &tuiMockLifecycle{}, actRestart); !errors.Is(err, want) {
		t.Fatalf("restart error = %v, want %v", err, want)
	}
}

func TestTUIRestartCallsServiceControl(t *testing.T) {
	ctrl := &tuiMockControl{}

	if err := runActionCmd(ctrl, &tuiMockLifecycle{}, actRestart); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ctrl.restartCalled {
		t.Error("Restart should be called")
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "off"},
		{time.Hour, "1h"},
		{6 * time.Hour, "6h"},
		{24 * time.Hour, "24h"},
		{90 * time.Minute, "1h30m"},
		{45 * time.Minute, "45m"},
		{30 * time.Second, "30s"},
	}
	for _, tc := range cases {
		if got := formatDuration(tc.d); got != tc.want {
			t.Fatalf("formatDuration(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}

func TestWindowLinesClampsOffset(t *testing.T) {
	lines := []string{"a", "b", "c", "d", "e"}
	visible, offset := windowLines(lines, 2, 2)
	if offset != 2 || strings.Join(visible, ",") != "c,d" {
		t.Fatalf("windowLines(offset 2) = %v at %d, want c,d at 2", visible, offset)
	}
	visible, offset = windowLines(lines, 99, 2)
	if offset != 3 || strings.Join(visible, ",") != "d,e" {
		t.Fatalf("windowLines(offset 99) = %v at %d, want d,e at 3", visible, offset)
	}
	visible, offset = windowLines(lines, -5, 2)
	if offset != 0 || strings.Join(visible, ",") != "a,b" {
		t.Fatalf("windowLines(offset -5) = %v at %d, want a,b at 0", visible, offset)
	}
	if visible, offset := windowLines(nil, 3, 2); visible != nil || offset != 0 {
		t.Fatalf("windowLines(no lines) = %v at %d, want nil at 0", visible, offset)
	}
}

func TestTUITickRunsOnlyWhileExecuting(t *testing.T) {
	m := model{}
	if _, cmd := m.Update(tickMsg{}); cmd != nil {
		t.Fatal("idle tick should not reschedule")
	}
	m.executing = actInstall
	updated, cmd := m.Update(tickMsg{})
	if cmd == nil {
		t.Fatal("running tick should reschedule")
	}
	if updated.(model).spinnerFrame != 1 {
		t.Fatalf("spinner frame = %d, want 1", updated.(model).spinnerFrame)
	}
}

func TestTUIPreviewScrollsAndResetsOnRefresh(t *testing.T) {
	m := model{
		mode:           modeConfig,
		configTab:      configTabPreview,
		previewContent: "1\n2\n3\n4\n5",
		height:         8,
	}
	down := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	updated, _ := m.Update(down)
	updated, _ = updated.(model).Update(down)
	if got := updated.(model).previewOffset; got != 2 {
		t.Fatalf("previewOffset after two scrolls = %d, want 2", got)
	}
	updated, _ = updated.(model).Update(down)
	if got := updated.(model).previewOffset; got != 2 {
		t.Fatalf("previewOffset should clamp at the last window, got %d", got)
	}
	up := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
	updated, _ = updated.(model).Update(up)
	if got := updated.(model).previewOffset; got != 1 {
		t.Fatalf("previewOffset after scroll up = %d, want 1", got)
	}
	updated, _ = updated.(model).Update(configPreviewMsg{content: "only"})
	if got := updated.(model).previewOffset; got != 0 {
		t.Fatalf("refresh should reset the preview offset, got %d", got)
	}
}

func TestTUIStatusNotInstalledShowsInstallOnly(t *testing.T) {
	m := model{status: &manager.Status{Installed: false}, width: 80}
	view := m.statusView()
	if !strings.Contains(view, "not installed") {
		t.Fatalf("status view = %q, want not-installed state", view)
	}
	if strings.Contains(view, "version:") {
		t.Fatalf("not-installed card should hide version, got %q", view)
	}
	if !strings.Contains(view, "i) Install") {
		t.Fatalf("status view should offer install, got %q", view)
	}
}

func TestTUIStatusCardStaysAlignedWithLongVersion(t *testing.T) {
	long := "v1.19.13-custom-build-20260101-abcdef123456"
	m := model{status: &manager.Status{Installed: true, InstanceState: manager.Running, Version: long}}
	view := m.statusView()
	if !strings.Contains(view, long) {
		t.Fatalf("status view should show the full version, got %q", view)
	}
	var widths []int
	for _, line := range strings.Split(view, "\n") {
		if strings.ContainsAny(line, "╭│╰") {
			widths = append(widths, lipgloss.Width(line))
		}
	}
	if len(widths) < 3 {
		t.Fatalf("expected a bordered card in %q", view)
	}
	for _, w := range widths {
		if w != widths[0] {
			t.Fatalf("card lines misaligned: %v", widths)
		}
	}
}

func TestTUIStatusErrorViewOffersRetry(t *testing.T) {
	m := model{ready: true, statusErr: errors.New("status lookup failed")}
	view := m.View()
	if !strings.Contains(view, "status lookup failed") {
		t.Fatalf("error view = %q, want the error text", view)
	}
	if !strings.Contains(view, "r) Retry") {
		t.Fatalf("error view = %q, want a retry hint", view)
	}
}

func TestTUIPreviewErrorIsShown(t *testing.T) {
	m := model{configTab: configTabPreview, previewErr: errors.New("preview fetch failed")}
	view := m.configView()
	if !strings.Contains(view, "preview fetch failed") {
		t.Fatalf("preview view = %q, want the fetch error", view)
	}
	if !strings.Contains(view, "r) Retry") {
		t.Fatalf("preview view = %q, want a retry hint", view)
	}
}
