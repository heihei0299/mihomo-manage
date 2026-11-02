package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"github.com/heihei0299/mihomo-manage/internal/manager"
)

type tuiMockControl struct {
	startCalled   bool
	restartCalled bool
	reloadCalled  bool
	startErr      error
	restartErr    error
	reloadErr     error
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

func (m *tuiMockControl) Reload(ctx context.Context) error {
	m.reloadCalled = true
	return m.reloadErr
}

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

func (m *tuiMockConfig) FetchSubscription(ctx context.Context) (bool, error) { return true, nil }

func (m *tuiMockConfig) ApplyConfig(ctx context.Context) error {
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

	if got := m.statusView(); !strings.Contains(got, "Config: apply-failed") {
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

	if got := m.statusView(); !strings.Contains(got, "Schedule: legacy") {
		t.Fatalf("status view = %q, want legacy schedule status", got)
	}
}

func TestTUIConfigViewShowsSubscriptionEditError(t *testing.T) {
	m := model{configTab: configTabSubscription, execResult: "failed", resultLabel: "Subscription source update", actionErr: errors.New("config update failed")}
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

func TestTUIReloadLoadingFeedback(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"success", nil, "✓ Reload succeeded"},
		{"failure", errors.New("reload rejected"), "✗ Reload failed: reload rejected"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := &tuiMockControl{reloadErr: tc.err}
			m := model{
				ready: true, width: 80, height: 24,
				status:  &manager.Status{Installed: true, InstanceState: manager.Running},
				control: ctrl, config: &tuiMockConfig{},
			}
			updated, dispatch := m.Update(tuiKey("4"))
			m = updated.(model)
			if dispatch == nil || m.executing != actReload {
				t.Fatal("4 should start Reload")
			}
			if view := ansi.Strip(m.View()); !strings.Contains(view, "mihomo-manager · Reload") || !strings.Contains(view, "Reloading mihomo configuration...") {
				t.Errorf("Reload should show a concrete loading page:\n%s", view)
			}

			// An immediately completed service call must not erase the loading page
			// before the renderer can paint it. The business operation is not delayed.
			done := execActionCmd(ctrl, nil, actReload, nil, "", false)()
			updated, finishFeedback := m.Update(done)
			m = updated.(model)
			if !ctrl.reloadCalled || m.executing != actReload || m.execResult != "" || finishFeedback == nil {
				t.Fatal("fast Reload completion should keep loading feedback visible, not immediately return to Status")
			}
			updated, next := m.Update(tickMsg{})
			m = updated.(model)
			if m.executing != actReload || next == nil {
				t.Fatal("loading feedback should keep animating until its display interval elapses")
			}
			for _, size := range [][2]int{{80, 24}, {60, 16}} {
				m.width, m.height = size[0], size[1]
				view := ansi.Strip(m.View())
				assertFrame(t, view, size[0], size[1])
				if !strings.Contains(view, "Reloading mihomo configuration...") {
					t.Fatalf("loading description missing at %dx%d:\n%s", size[0], size[1], view)
				}
			}

			completion := finishFeedback()
			if _, ok := completion.(actionDoneMsg); !ok {
				t.Fatalf("loading feedback should defer the original completion, got %T", completion)
			}
			if time.Since(m.startedAt) < reloadFeedbackMinDuration {
				t.Fatal("Reload loading page disappeared before its minimum display interval")
			}
			updated, refresh := m.Update(completion)
			m = updated.(model)
			if m.executing != actNone || refresh == nil {
				t.Fatal("completed Reload should leave the loading page and refresh status after its display interval")
			}
			if view := ansi.Strip(m.View()); !strings.Contains(view, tc.want) {
				t.Fatalf("loading completion lost the command result:\n%s", view)
			}
			updated, _ = m.Update(refresh())
			if view := ansi.Strip(updated.View()); !strings.Contains(view, tc.want) {
				t.Fatalf("status refresh lost the Reload result:\n%s", view)
			}
		})
	}
}

func TestTUIReloadFeedback(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{"success", nil, "✓ Reload succeeded"},
		{"failure", errors.New("permission denied\nreload diagnostic"), "✗ Reload failed: permission denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := &tuiMockControl{reloadErr: tc.err}
			m := model{
				ready: true, width: 80, height: 24,
				status:  &manager.Status{Installed: true, InstanceState: manager.Running},
				control: ctrl, config: &tuiMockConfig{},
			}
			updated, dispatch := m.Update(tuiKey("4"))
			m = updated.(model)
			if m.executing != actReload || dispatch == nil {
				t.Fatal("4 should dispatch Reload")
			}
			// Exercise an operation that has already outlasted the loading interval.
			m.startedAt = time.Now().Add(-time.Second)
			done := execActionCmd(ctrl, nil, actReload, nil, "", false)()
			updated, refresh := m.Update(done)
			m = updated.(model)
			if !ctrl.reloadCalled || refresh == nil {
				t.Fatal("Reload should execute and request a status refresh")
			}
			assertFeedback := func(stage string, m model) {
				t.Helper()
				if view := ansi.Strip(m.View()); !strings.Contains(view, tc.want) {
					t.Errorf("%s should retain %q:\n%s", stage, tc.want, view)
				}
			}
			assertFeedback("completion", m)
			updated, _ = m.Update(refresh())
			m = updated.(model)
			assertFeedback("automatic status refresh", m)
			for _, event := range []tea.Msg{tickMsg{}, tuiKey("down"), tuiKey("1")} {
				updated, _ = m.Update(event)
				m = updated.(model)
				assertFeedback("idle tick, scrolling or unavailable action", m)
			}
			updated, refresh = m.Update(tuiKey("r"))
			updated, _ = updated.(model).Update(refresh())
			m = updated.(model)
			assertFeedback("manual status refresh", m)
			updated, _ = m.Update(tuiKey("c"))
			assertFeedback("configuration view", updated.(model))
			updated, _ = updated.(model).Update(tuiKey("q"))
			m = updated.(model)
			assertFeedback("return from configuration", m)
			updated, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 16})
			m = updated.(model)
			assertFeedback("compact terminal", m)

			failedRefresh, _ := m.Update(statusMsg{err: errors.New("status lookup failed")})
			assertFeedback("failed status refresh", failedRefresh.(model))
			if !strings.Contains(ansi.Strip(failedRefresh.View()), "status lookup failed") {
				t.Error("status refresh failure should remain readable alongside the command result")
			}
			if tc.err != nil && !strings.Contains(ansi.Strip(failedRefresh.View()), "reload diagnostic") {
				t.Error("the complete command error should survive a failed status refresh")
			}

			updated, _ = m.Update(tuiKey("3"))
			m = updated.(model)
			if m.executing != actRestart || m.execResult != "" || m.actionErr != nil {
				t.Error("starting the next command should clear the previous result")
			}
			updated, _ = m.Update(execActionCmd(ctrl, nil, actRestart, nil, "", false)())
			if view := ansi.Strip(updated.View()); !strings.Contains(view, "✓ Restart succeeded") || strings.Contains(view, tc.want) {
				t.Errorf("the next command should replace the previous result:\n%s", view)
			}
		})
	}
}

func TestTUIActionFeedbackNamesCommand(t *testing.T) {
	for _, tc := range []struct {
		a    action
		name string
	}{
		{actStart, "Start"}, {actStop, "Stop"}, {actRestart, "Restart"},
		{actReload, "Reload"}, {actInstall, "Install"}, {actUpgrade, "Upgrade"},
		{actUninstall, "Uninstall"}, {actAutostartOn, "Enable autostart"},
		{actAutostartOff, "Disable autostart"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, err := range []error{nil, errors.New("command error")} {
				m := model{ready: true, status: &manager.Status{Installed: true}}
				updated, _ := m.Update(actionDoneMsg{action: tc.a, err: err})
				want := "✓ " + tc.name + " succeeded"
				if err != nil {
					want = "✗ " + tc.name + " failed: command error"
				}
				if view := ansi.Strip(updated.View()); !strings.Contains(view, want) {
					t.Errorf("command result should contain %q:\n%s", want, view)
				}
			}
		})
	}
}

func TestTUISettingsFeedbackNamesCommand(t *testing.T) {
	for _, tc := range []struct {
		name    string
		message func(error) tea.Msg
	}{
		{"Schedule update", func(err error) tea.Msg { return scheduleDoneMsg{err: err} }},
		{"Subscription source update", func(err error) tea.Msg { return subscriptionEditMsg{err: err} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, err := range []error{nil, errors.New("command error")} {
				m := model{
					ready: true, mode: modeConfig,
					status: &manager.Status{Installed: true},
				}
				updated, _ := m.Update(tc.message(err))
				want := "✓ " + tc.name + " succeeded"
				if err != nil {
					want = "✗ " + tc.name + " failed: command error"
				}
				if view := ansi.Strip(updated.View()); !strings.Contains(view, want) {
					t.Errorf("settings result should contain %q:\n%s", want, view)
				}
			}
		})
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
	lines := make([]string, 40)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%02d", i+1)
	}
	m := model{
		ready:          true,
		mode:           modeConfig,
		configTab:      configTabPreview,
		previewContent: strings.Join(lines, "\n"),
		width:          80,
		height:         24,
	}
	down := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")}
	updated, _ := m.Update(down)
	updated, _ = updated.(model).Update(down)
	if got := updated.(model).previewOffset; got != 2 {
		t.Fatalf("previewOffset after two scrolls = %d, want 2", got)
	}
	updated, _ = updated.(model).Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if got := updated.(model).previewOffset; got <= 2 {
		t.Fatalf("previewOffset after page down = %d, want a full page further", got)
	}
	updated, _ = updated.(model).Update(tea.KeyMsg{Type: tea.KeyPgDown})
	if got := updated.(model).previewOffset; got > 40 {
		t.Fatalf("previewOffset should stay inside the content, got %d", got)
	}
	up := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")}
	updated, _ = updated.(model).Update(up)
	if got := updated.(model).previewOffset; got <= 0 {
		t.Fatalf("previewOffset after scroll up = %d, want a positive offset", got)
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
	if strings.Contains(view, "Core:") {
		t.Fatalf("not-installed card should hide version, got %q", view)
	}
	if !strings.Contains(view, "i) Install") {
		t.Fatalf("status view should offer install, got %q", view)
	}
}

func TestTUIStatusOverviewPreservesLongVersion(t *testing.T) {
	long := "v1.19.13-custom-build-20260101-abcdef123456"
	m := model{status: &manager.Status{Installed: true, InstanceState: manager.Running, Version: long}}
	view := m.statusView()
	if !strings.Contains(view, long) {
		t.Fatalf("status view should show the full version, got %q", view)
	}
	assertFrame(t, view, 80, 24)
	if strings.ContainsAny(view, "╭│╰") {
		t.Fatalf("status overview should not draw a large border:\n%s", view)
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

// ---------------------------------------------------------------------------
// Layout and wrapping
// ---------------------------------------------------------------------------

func tuiKey(name string) tea.KeyMsg {
	switch name {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
}

// assertFrame checks the invariants every rendered page must satisfy.
func assertFrame(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		t.Fatalf("frame has %d lines, want <= %d:\n%s", len(lines), height, view)
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w > width {
			t.Fatalf("line %d is %d cells wide, want <= %d: %q\n%s", i, w, width, line, view)
		}
	}
}

func tuiPageSamples() []struct {
	name string
	m    model
	want []string
} {
	running := &manager.Status{Installed: true, InstanceState: manager.Running, Version: "v1.19.0", AutoStartEnabled: true}
	return []struct {
		name string
		m    model
		want []string
	}{
		{"status", model{ready: true, status: running}, []string{"● running", "r) Refresh", "q) Quit"}},
		{"config-subscription", model{ready: true, status: running, mode: modeConfig, configTab: configTabSubscription}, []string{"Subscription source", "e) Edit", "q) Back"}},
		{"config-override", model{ready: true, status: running, mode: modeConfig, configTab: configTabOverride}, []string{manager.OverrideFilePath, "q) Back"}},
		{"config-preview", model{ready: true, status: running, mode: modeConfig, configTab: configTabPreview, previewContent: strings.Repeat("    key: value\n", 40)}, []string{"key: value", "q) Back"}},
		{"config-preview-loading", model{ready: true, status: running, mode: modeConfig, configTab: configTabPreview, previewLoading: true}, []string{"Loading preview...", "q) Back"}},
		{"config-preview-empty", model{ready: true, status: running, mode: modeConfig, configTab: configTabPreview}, []string{"(empty)", "r) Refresh", "q) Back"}},
		{"config-preview-error", model{ready: true, status: running, mode: modeConfig, configTab: configTabPreview, previewErr: errors.New("preview fetch failed")}, []string{"preview fetch failed", "r) Retry", "q) Back"}},
		{"versions", model{ready: true, status: running, mode: modeChooseVersion, versions: []manager.VersionInfo{{Tag: "v1.19.0"}, {Tag: "v1.19.1"}}}, []string{"Select version", "q) cancel"}},
		{"schedule", model{ready: true, status: running, mode: modeSchedule, scheduleInterval: time.Hour, scheduleActive: true}, []string{"current: every 1h", "q) cancel"}},
		{"uninstall", model{ready: true, status: running, mode: modeConfirmUninstall}, []string{"Uninstall mihomo?", "y) Confirm", "n) Cancel"}},
		{"executing", model{ready: true, status: running, executing: actUpgrade, phaseLabel: "fetch", phaseMsg: "downloading mihomo"}, []string{"upgrading...", "[fetch] downloading mihomo"}},
		{"loading", model{}, []string{"Checking mihomo status...", "q) Quit"}},
		{"status-error", model{ready: true, statusErr: errors.New("status lookup failed")}, []string{"status lookup failed", "r) Retry", "q) Quit"}},
	}
}

func TestTUIPageFramesFitEverySize(t *testing.T) {
	for _, size := range [][2]int{{120, 40}, {80, 24}, {60, 16}} {
		for _, sample := range tuiPageSamples() {
			m := sample.m
			m.width, m.height = size[0], size[1]
			view := m.View()
			assertFrame(t, view, size[0], size[1])
			for _, want := range sample.want {
				if !strings.Contains(view, want) {
					t.Fatalf("%s at %dx%d is missing %q:\n%s", sample.name, size[0], size[1], want, view)
				}
			}
		}
	}
}

func TestTUISmallTerminalShowsResizeHint(t *testing.T) {
	for _, size := range [][2]int{{59, 16}, {60, 15}, {40, 10}, {20, 5}} {
		for _, sample := range tuiPageSamples() {
			m := sample.m
			m.width, m.height = size[0], size[1]
			view := m.View()
			assertFrame(t, view, size[0], size[1])
			if !strings.Contains(view, "Terminal too small") {
				t.Fatalf("%s at %dx%d should show the resize hint:\n%s", sample.name, size[0], size[1], view)
			}
		}
	}
	for _, size := range [][2]int{{10, 3}, {5, 1}} {
		m := model{ready: true, status: &manager.Status{Installed: true, InstanceState: manager.Running}}
		m.width, m.height = size[0], size[1]
		assertFrame(t, m.View(), size[0], size[1])
	}
}

func TestTUIFrameStartsAtStandardBudget(t *testing.T) {
	m := model{}
	view := m.View()
	if strings.Contains(view, "Terminal too small") {
		t.Fatalf("first frame should render at the 80x24 budget, got %q", view)
	}
	if !strings.Contains(view, "Checking mihomo status...") {
		t.Fatalf("first frame should show the loading state, got %q", view)
	}
	assertFrame(t, view, 80, 24)
}

func TestTUIWrapTextPreservesContent(t *testing.T) {
	cases := []struct {
		name  string
		line  string
		width int
	}{
		{"long-url", "https://example.com/" + strings.Repeat("segment/", 20), 20},
		{"long-path", "/opt/mihomo/etc/" + strings.Repeat("nested/", 20) + "config.yaml", 16},
		{"unicode", strings.Repeat("配置项中文测试", 8), 12},
		{"combining", "e\u0301" + strings.Repeat("é", 30), 7},
		{"tabs", "\t\tkey:\tvalue", 10},
		{"blank", "", 10},
		{"indent", "      - name: " + strings.Repeat("long-value-", 10), 24},
	}
	for _, tc := range cases {
		parts := wrapText(tc.line, tc.width)
		if len(parts) == 0 {
			t.Fatalf("%s: wrapText returned no lines", tc.name)
		}
		var rebuilt strings.Builder
		for _, part := range parts {
			if w := ansi.StringWidth(part); w > tc.width {
				t.Fatalf("%s: line %q is %d cells wide, want <= %d", tc.name, part, w, tc.width)
			}
			rebuilt.WriteString(part)
		}
		got := strings.Join(strings.Fields(rebuilt.String()), "")
		want := strings.Join(strings.Fields(strings.ReplaceAll(tc.line, "\t", "    ")), "")
		if got != want {
			t.Fatalf("%s: content changed after wrapping\n got %q\nwant %q", tc.name, got, want)
		}
	}
	if got, want := strings.Join(wrapText("      keep-indent", 40), "|"), "      keep-indent"; got != want {
		t.Fatalf("short indented line = %q, want %q", got, want)
	}
}

func TestTUIWrapTextHandlesANSI(t *testing.T) {
	plain := strings.Repeat("styled-word-", 8)
	styled := "\x1b[31m" + plain + "\x1b[0m"
	parts := wrapText(styled, 20)
	for _, part := range parts {
		if w := ansi.StringWidth(part); w > 20 {
			t.Fatalf("styled line %q is %d cells wide, want <= 20", part, w)
		}
	}
	got := strings.Join(strings.Fields(ansi.Strip(strings.Join(parts, "\n"))), "")
	want := strings.Join(strings.Fields(plain), "")
	if got != want {
		t.Fatalf("styled content changed: got %q, want %q", got, want)
	}
}

func TestTUIStatusOverviewWrapsLongVersions(t *testing.T) {
	long := "v1.19.13-" + strings.Repeat("segment-", 20) + "end"
	for _, size := range [][2]int{{120, 40}, {80, 24}, {60, 16}} {
		m := model{
			ready:  true,
			status: &manager.Status{Installed: true, InstanceState: manager.Running, Version: long},
			width:  size[0], height: size[1],
		}
		view := m.View()
		assertFrame(t, view, size[0], size[1])
		compact := strings.Join(strings.Fields(ansi.Strip(view)), "")
		if !strings.Contains(compact, long) {
			t.Fatalf("%dx%d: status view lost part of the version:\n%s", size[0], size[1], view)
		}
		if strings.ContainsAny(view, "╭│╰") {
			t.Fatalf("%dx%d: overview should not draw a border:\n%s", size[0], size[1], view)
		}
	}
}

func TestTUIPreviewWrapsAndMapsSourceLines(t *testing.T) {
	content := "key: " + strings.Repeat("value-", 30) + "\nsecond: line\n\nthird: line"
	before := content
	screen, origin := wrapPreview(content, 30)
	if len(screen) <= 4 {
		t.Fatalf("preview did not wrap: %d screen lines for 4 source lines", len(screen))
	}
	if len(screen) != len(origin) {
		t.Fatalf("screen lines = %d, origins = %d", len(screen), len(origin))
	}
	for i, line := range origin {
		if line < 0 || line > 3 {
			t.Fatalf("screen line %d maps to source line %d, want 0..3", i, line)
		}
		if i > 0 && line < origin[i-1] {
			t.Fatalf("source line mapping is not monotonic: %v", origin)
		}
	}
	if got, want := rebuildPreviewSource(screen, origin), content; got != want {
		t.Fatalf("preview content changed\n got %q\nwant %q", got, want)
	}
	if content != before {
		t.Fatalf("preview source was modified: %q", content)
	}
	m := model{ready: true, mode: modeConfig, configTab: configTabPreview, previewContent: content, width: 30, height: 16}
	_ = m.View()
	if m.previewContent != before {
		t.Fatalf("rendering modified the stored preview content: %q", m.previewContent)
	}
}

func TestTUIPreviewFooterReportsSourceRange(t *testing.T) {
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%03d-%s", i+1, strings.Repeat("x", 100))
	}
	content := strings.Join(lines, "\n")
	m := model{ready: true, mode: modeConfig, configTab: configTabPreview, previewContent: content, width: 80, height: 24}
	if view := m.View(); !strings.Contains(view, "lines 1-") {
		t.Fatalf("preview footer should start at source line 1:\n%s", view)
	}
	for i := 0; i < 3; i++ {
		updated, _ := m.Update(tuiKey("j"))
		m = updated.(model)
	}
	if m.previewOffset != 3 {
		t.Fatalf("previewOffset = %d, want 3", m.previewOffset)
	}
	_, origin := wrapPreview(content, 80)
	if got := origin[m.previewOffset]; got != 1 {
		t.Fatalf("top source line after scrolling = %d, want 1", got)
	}
	if view := m.View(); !strings.Contains(view, fmt.Sprintf("lines %d-", origin[m.previewOffset]+1)) {
		t.Fatalf("preview footer should report source lines from %d:\n%s", origin[m.previewOffset]+1, view)
	}
}

// ---------------------------------------------------------------------------
// Interaction regressions
// ---------------------------------------------------------------------------

func tuiPreviewContent(lines int, width int) string {
	out := make([]string, lines)
	for i := range out {
		out[i] = fmt.Sprintf("line-%03d-%s", i+1, strings.Repeat("y", width))
	}
	return strings.Join(out, "\n")
}

func TestTUIPreviewResizeKeepsTopSourceLine(t *testing.T) {
	content := tuiPreviewContent(100, 100)
	m := model{ready: true, mode: modeConfig, configTab: configTabPreview, previewContent: content, width: 80, height: 24}
	for i := 0; i < 7; i++ {
		updated, _ := m.Update(tuiKey("j"))
		m = updated.(model)
	}
	_, origin := wrapPreview(content, 80)
	top := origin[m.previewOffset]

	for _, size := range [][2]int{{60, 24}, {120, 24}} {
		updated, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		resized := updated.(model)
		_, resizedOrigin := wrapPreview(content, size[0])
		if got := resizedOrigin[resized.previewOffset]; got != top {
			t.Fatalf("%dx%d: top source line after resize = %d, want %d", size[0], size[1], got, top)
		}
		if view := resized.View(); !strings.Contains(view, fmt.Sprintf("lines %d-", top+1)) {
			t.Fatalf("%dx%d: footer should start at source line %d:\n%s", size[0], size[1], top+1, view)
		}
		assertFrame(t, resized.View(), size[0], size[1])
	}
}

func TestTUIPreviewResizeClampsReadingOffset(t *testing.T) {
	content := tuiPreviewContent(100, 100)
	m := model{ready: true, mode: modeConfig, configTab: configTabPreview, previewContent: content, width: 80, height: 24}
	for i := 0; i < 200; i++ {
		updated, _ := m.Update(tuiKey("j"))
		m = updated.(model)
	}
	tall, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	resized := tall.(model)
	info := resized.currentFrame()
	if !info.valid {
		t.Fatal("resized preview should report a body window")
	}
	if resized.previewOffset != info.offset {
		t.Fatalf("previewOffset = %d, want the clamped offset %d", resized.previewOffset, info.offset)
	}
	if max := info.total - info.bodyHeight; resized.previewOffset > max {
		t.Fatalf("previewOffset = %d, want <= %d", resized.previewOffset, max)
	}
	assertFrame(t, resized.View(), 80, 40)
}

func TestTUIStatusReadScrollsOverflow(t *testing.T) {
	long := strings.Repeat("v-long-segment-", 200)
	m := model{
		ready:  true,
		status: &manager.Status{Installed: true, InstanceState: manager.Running, Version: long},
		width:  60,
		height: 16,
	}
	view := m.View()
	assertFrame(t, view, 60, 16)
	if !strings.Contains(view, "scroll") {
		t.Fatalf("overflowing status page should advertise scrolling:\n%s", view)
	}
	updated, _ := m.Update(tuiKey("j"))
	if got := updated.(model).readOffset; got != 1 {
		t.Fatalf("readOffset after j = %d, want 1", got)
	}
	updated, _ = updated.(model).Update(tuiKey("pgdown"))
	m = updated.(model)
	if m.readOffset <= 1 {
		t.Fatalf("readOffset after page down = %d, want more than one line", m.readOffset)
	}
	for i := 0; i < 100; i++ {
		updated, _ := m.Update(tuiKey("pgdown"))
		m = updated.(model)
	}
	info := m.currentFrame()
	if info.total <= info.bodyHeight {
		t.Fatal("expected overflowing status content")
	}
	if max := info.total - info.bodyHeight; m.readOffset > max {
		t.Fatalf("readOffset = %d, want <= %d", m.readOffset, max)
	}
	assertFrame(t, m.View(), 60, 16)
}

func TestTUIListKeepsSelectionVisible(t *testing.T) {
	versions := make([]manager.VersionInfo, 30)
	for i := range versions {
		versions[i] = manager.VersionInfo{Tag: fmt.Sprintf("v0.%d.0", i+1)}
	}
	m := model{
		ready:       true,
		status:      &manager.Status{Installed: true, InstanceState: manager.Running, Version: "v0.1.0"},
		mode:        modeChooseVersion,
		versions:    versions,
		selectedIdx: 29,
		width:       60,
		height:      16,
	}
	view := m.View()
	assertFrame(t, view, 60, 16)
	if !strings.Contains(view, "> v0.30.0") {
		t.Fatalf("selected version should stay visible:\n%s", view)
	}
	m.selectedIdx = 0
	if view := m.View(); !strings.Contains(view, "> v0.1.0") {
		t.Fatalf("first version should be visible after moving up:\n%s", view)
	}

	schedule := model{ready: true, status: m.status, mode: modeSchedule, selectedIdx: 4, width: 60, height: 16}
	if view := schedule.View(); !strings.Contains(view, "> 24h") {
		t.Fatalf("selected schedule preset should stay visible:\n%s", view)
	}
}

func TestTUISmallTerminalRefusesBusinessKeys(t *testing.T) {
	running := &manager.Status{Installed: true, InstanceState: manager.Running}
	base := model{
		ready:     true,
		status:    running,
		width:     59,
		height:    16,
		control:   &tuiMockControl{},
		lifecycle: &tuiMockLifecycle{},
		config:    &tuiMockConfig{},
	}
	for _, key := range []string{"1", "2", "3", "4", "5", "c", "a", "t", "u", "i", "r"} {
		updated, cmd := base.Update(tuiKey(key))
		if cmd != nil {
			t.Fatalf("key %q should not run anything on a too-small terminal", key)
		}
		got := updated.(model)
		if got.mode != modeStatus || got.executing != actNone {
			t.Fatalf("key %q should be ignored on a too-small terminal (mode=%v executing=%v)", key, got.mode, got.executing)
		}
	}
	if _, cmd := base.Update(tuiKey("q")); cmd == nil {
		t.Fatal("q should still quit from the status page")
	}

	config := base
	config.mode = modeConfig
	config.configTab = configTabPreview
	updated, cmd := config.Update(tuiKey("j"))
	if cmd != nil || updated.(model).previewOffset != 0 {
		t.Fatal("scrolling should be ignored on a too-small terminal")
	}
	if updated, cmd = config.Update(tuiKey("e")); cmd != nil {
		t.Fatal("the editor should not open on a too-small terminal")
	}
	if updated, _ = updated.(model).Update(tuiKey("q")); updated.(model).mode != modeStatus {
		t.Fatal("q should still leave the config page")
	}

	uninstall := base
	uninstall.mode = modeConfirmUninstall
	updated, _ = uninstall.Update(tuiKey("y"))
	if updated.(model).executing != actNone {
		t.Fatal("uninstall should not start on a too-small terminal")
	}
	if updated.(model).keepBackup {
		t.Fatal("b should not toggle the backup flag on a too-small terminal")
	}
	if updated, _ = updated.(model).Update(tuiKey("n")); updated.(model).mode != modeStatus {
		t.Fatal("n should still cancel the uninstall dialog")
	}
}

func TestTUIKeysStillNavigate(t *testing.T) {
	running := &manager.Status{Installed: true, InstanceState: manager.Running}
	m := model{
		ready:     true,
		status:    running,
		width:     80,
		height:    24,
		control:   &tuiMockControl{},
		lifecycle: &tuiMockLifecycle{},
		config:    &tuiMockConfig{},
	}
	updated, cmd := m.Update(tuiKey("c"))
	if updated.(model).mode != modeConfig || cmd == nil {
		t.Fatal("c should open the config page and load the preview")
	}
	if updated, _ = updated.(model).Update(tuiKey("q")); updated.(model).mode != modeStatus {
		t.Fatal("q should leave the config page")
	}
	if updated, _ = m.Update(tuiKey("5")); updated.(model).mode != modeChooseVersion {
		t.Fatal("5 should open the version picker")
	}
	if updated, _ = updated.(model).Update(tuiKey("esc")); updated.(model).mode != modeStatus {
		t.Fatal("esc should leave the version picker")
	}
	if updated, _ = m.Update(tuiKey("t")); updated.(model).mode != modeSchedule {
		t.Fatal("t should open the schedule page")
	}
	if updated, _ = m.Update(tuiKey("u")); updated.(model).mode != modeConfirmUninstall {
		t.Fatal("u should open the uninstall dialog")
	}

	confirm := model{
		ready:     true,
		status:    running,
		mode:      modeConfirmUninstall,
		width:     80,
		height:    24,
		control:   &tuiMockControl{},
		lifecycle: &tuiMockLifecycle{},
		config:    &tuiMockConfig{},
	}
	updated, _ = confirm.Update(tuiKey("b"))
	if !updated.(model).keepBackup {
		t.Fatal("b should toggle the backup flag")
	}
	if updated, _ = updated.(model).Update(tuiKey("n")); updated.(model).mode != modeStatus {
		t.Fatal("n should cancel the uninstall dialog")
	}
	if updated, _ = confirm.Update(tuiKey("y")); updated.(model).executing != actUninstall {
		t.Fatal("y should start the uninstall")
	}
}

func TestTUIProgressKeepsPhaseAndElapsed(t *testing.T) {
	m := model{
		ready:      true,
		executing:  actUpgrade,
		phaseLabel: "fetch",
		phaseMsg:   strings.Repeat("downloading mihomo ", 100),
		startedAt:  time.Now().Add(-3 * time.Second),
		width:      60,
		height:     16,
	}
	view := m.View()
	assertFrame(t, view, 60, 16)
	for _, want := range []string{"upgrading...", "[fetch]", "elapsed"} {
		if !strings.Contains(view, want) {
			t.Fatalf("progress view should show %q:\n%s", want, view)
		}
	}
	lines := strings.Split(view, "\n")
	last := strings.TrimRight(ansi.Strip(lines[len(lines)-1]), " ")
	if !strings.Contains(last, "elapsed") {
		t.Fatalf("elapsed time must stay on the last line, got %q:\n%s", last, view)
	}
	clipped := false
	for _, line := range lines {
		if strings.HasSuffix(strings.TrimRight(ansi.Strip(line), " "), "…") {
			clipped = true
		}
	}
	if !clipped {
		t.Fatalf("clipped phase message should end with an ellipsis:\n%s", view)
	}
}

func TestTUIPageSwitchResetsReadingOffset(t *testing.T) {
	long := strings.Repeat("v-long-segment-", 200)
	m := model{
		ready:  true,
		status: &manager.Status{Installed: true, InstanceState: manager.Running, Version: long},
		width:  60,
		height: 16,
		config: &tuiMockConfig{},
	}
	updated, _ := m.Update(tuiKey("j"))
	m = updated.(model)
	if m.readOffset == 0 {
		t.Fatal("expected the status page to scroll")
	}
	if updated, _ = m.Update(tuiKey("c")); updated.(model).readOffset != 0 {
		t.Fatal("opening the config page should reset the reading offset")
	}
	m.readOffset = 7
	if updated, _ = m.Update(statusMsg{status: m.status}); updated.(model).readOffset != 0 {
		t.Fatal("a status refresh should reset the reading offset")
	}
}

func TestTUIViewHasNoColourEscapeByDefault(t *testing.T) {
	m := model{
		ready:  true,
		status: &manager.Status{Installed: true, InstanceState: manager.Running, Version: "v1.0.0"},
		width:  80,
		height: 24,
	}
	view := m.View()
	if ansi.Strip(view) != view {
		t.Skip("terminal reports colour support; plain-text rendering is covered elsewhere")
	}
	if strings.Contains(view, "\x1b[") {
		t.Fatalf("plain rendering should not emit escapes:\n%q", view)
	}
}

func TestTUILongErrorsStayReadable(t *testing.T) {
	const marker = "ZZ-END-OF-ERROR-ZZ"
	details := strings.Repeat("upstream returned an unexpected response at line 4; ", 40) + marker
	running := &manager.Status{Installed: true, InstanceState: manager.Running}
	cases := []struct {
		name  string
		m     model
		check func(*testing.T, model)
	}{
		{
			name: "status-config",
			m: model{ready: true, status: running,
				configStatus: manager.ConfigApplyStatus{State: manager.ConfigValidationFailed, ErrorSummary: details}},
			check: func(t *testing.T, m model) {
				if m.readOffset == 0 {
					t.Fatalf("status details should scroll with the reading offset")
				}
			},
		},
		{
			name: "status-action",
			m: model{ready: true, status: running,
				execResult: "failed", resultLabel: "Reload", actionErr: errors.New(details)},
			check: func(t *testing.T, m model) {
				if m.readOffset == 0 {
					t.Fatalf("action details should scroll with the reading offset")
				}
			},
		},
		{
			name: "config",
			m: model{ready: true, mode: modeConfig, configTab: configTabSubscription,
				execResult: "failed", resultLabel: "Subscription source update", actionErr: errors.New(details)},
			check: func(t *testing.T, m model) {
				if m.readOffset == 0 {
					t.Fatalf("config error details should scroll with the reading offset")
				}
			},
		},
		{
			name: "preview",
			m: model{ready: true, mode: modeConfig, configTab: configTabPreview,
				previewErr: errors.New(details)},
			check: func(t *testing.T, m model) {
				if m.readOffset == 0 {
					t.Fatalf("a preview error should scroll with the reading offset")
				}
				if m.previewOffset != 0 {
					t.Fatalf("previewOffset = %d, want 0 for an error page", m.previewOffset)
				}
			},
		},
		{
			name: "versions",
			m: model{ready: true, mode: modeChooseVersion, status: running,
				versionsErr: errors.New(details)},
			check: func(t *testing.T, m model) {
				if m.readOffset == 0 {
					t.Fatalf("version error details should scroll with the reading offset")
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.m
			m.width, m.height = 60, 16
			view := m.View()
			assertFrame(t, view, 60, 16)
			if strings.Contains(strings.ReplaceAll(ansi.Strip(view), "\n", ""), marker) {
				t.Fatalf("long error should start at the top:\n%s", view)
			}
			for i := 0; i < 200; i++ {
				updated, _ := m.Update(tuiKey("pgdown"))
				m = updated.(model)
			}
			view = m.View()
			assertFrame(t, view, 60, 16)
			if !strings.Contains(strings.ReplaceAll(ansi.Strip(view), "\n", ""), marker) {
				t.Fatalf("scrolling should reach the end of the error:\n%s", view)
			}
			if tc.check != nil {
				tc.check(t, m)
			}
		})
	}
}

func TestUITruncateLineKeepsStyleWidth(t *testing.T) {
	line := "\x1b[31m" + strings.Repeat("x", 30) + "\x1b[0m"
	clipped := truncateLine(line, 10)
	if w := ansi.StringWidth(clipped); w > 10 {
		t.Fatalf("truncated line is %d cells wide, want <= 10: %q", w, clipped)
	}
	if !strings.Contains(clipped, "…") {
		t.Fatalf("truncated line should end with an ellipsis: %q", clipped)
	}
	if short := "\x1b[32mok\x1b[0m"; truncateLine(short, 10) != short {
		t.Fatalf("short styled line should be left untouched: %q", truncateLine(short, 10))
	}
}

// ---------------------------------------------------------------------------
// Second-round regressions
// ---------------------------------------------------------------------------

// rebuildPreviewSource joins the wrapped screen lines back into source lines so
// a preview can be compared byte for byte with its input.
func rebuildPreviewSource(screen []string, origin []int) string {
	var b strings.Builder
	previous := -1
	for i, line := range screen {
		if origin[i] != previous {
			if previous >= 0 {
				b.WriteByte('\n')
			}
			previous = origin[i]
		}
		b.WriteString(line)
	}
	return b.String()
}

func TestTUIPreviewKeepsWhitespaceExactly(t *testing.T) {
	content := strings.Join([]string{
		`    key: "a  value   with   runs"`,
		`      - name: "x  y"   # trailing comment`,
		"\tindented\tby\ttabs",
		"",
		"    ",
		`  long: "` + strings.Repeat("space filled ", 40) + `"`,
	}, "\n")
	before := content
	screen, origin := wrapPreview(content, 60)
	if got, want := rebuildPreviewSource(screen, origin), strings.ReplaceAll(content, "\t", "    "); got != want {
		t.Fatalf("preview lost whitespace\n got %q\nwant %q", got, want)
	}
	if content != before {
		t.Fatalf("preview source was modified: %q", content)
	}
	if len(origin) <= len(strings.Split(content, "\n")) {
		t.Fatalf("long line did not wrap: %d screen lines", len(origin))
	}
}

func TestTUIPreviewFooterMatchesVisibleLines(t *testing.T) {
	source := make([]string, 100)
	for i := range source {
		source[i] = fmt.Sprintf("line-%03d", i+1)
	}
	linePattern := regexp.MustCompile(`^line-(\d{3})$`)
	cases := []struct{ width, height, offset int }{
		{74, 23, 80},
		{74, 23, 0},
		{100, 40, 9},
		{60, 16, 95},
		{80, 24, 50},
	}
	for _, tc := range cases {
		m := model{
			ready:          true,
			mode:           modeConfig,
			configTab:      configTabPreview,
			previewContent: strings.Join(source, "\n"),
			previewOffset:  tc.offset,
			width:          tc.width,
			height:         tc.height,
		}
		view := m.View()
		assertFrame(t, view, tc.width, tc.height)

		var footer string
		var visible []int
		for _, raw := range strings.Split(view, "\n") {
			line := ansi.Strip(raw)
			if strings.HasPrefix(line, "lines ") {
				footer = line
			}
			if match := linePattern.FindStringSubmatch(strings.TrimSpace(line)); match != nil {
				var number int
				fmt.Sscanf(match[1], "%d", &number)
				visible = append(visible, number)
			}
		}
		if footer == "" {
			t.Fatalf("%dx%d offset %d should show a source range footer:\n%s", tc.width, tc.height, tc.offset, view)
		}
		var start, end, total int
		if _, err := fmt.Sscanf(footer, "lines %d-%d / %d", &start, &end, &total); err != nil {
			t.Fatalf("unparsable footer %q: %v", footer, err)
		}
		if total != len(source) {
			t.Fatalf("footer total = %d, want %d", total, len(source))
		}
		if len(visible) != end-start+1 {
			t.Fatalf("footer says %d-%d but %d content lines are visible:\n%s", start, end, len(visible), view)
		}
		for i, number := range visible {
			if number != start+i {
				t.Fatalf("visible lines are not contiguous from %d: %v", start, visible)
			}
		}
	}
}

func TestTUIVersionListKeepsSelectionWithLongVersion(t *testing.T) {
	versions := make([]manager.VersionInfo, 30)
	for i := range versions {
		versions[i] = manager.VersionInfo{Tag: fmt.Sprintf("v0.%d.0", i+1)}
	}
	m := model{
		ready:       true,
		status:      &manager.Status{Installed: true, InstanceState: manager.Running, Version: strings.Repeat("v-long-", 40)},
		mode:        modeChooseVersion,
		versions:    versions,
		selectedIdx: 0,
		width:       60,
		height:      16,
	}
	view := m.View()
	assertFrame(t, view, 60, 16)
	if !strings.Contains(view, "> v0.1.0") {
		t.Fatalf("first version should stay visible:\n%s", view)
	}
	installedLines := 0
	for _, line := range strings.Split(ansi.Strip(view), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "installed:") {
			installedLines++
		}
	}
	if installedLines != 1 {
		t.Fatalf("installed summary should stay on one line, saw %d:\n%s", installedLines, view)
	}
	m.selectedIdx = 29
	view = m.View()
	if !strings.Contains(view, "> v0.30.0") {
		t.Fatalf("last version should stay visible:\n%s", view)
	}
	m.selectedIdx = 15
	view = m.View()
	if !strings.Contains(view, "> v0.16.0") {
		t.Fatalf("middle selection should stay visible:\n%s", view)
	}
}

func TestTUIResizeHintKeepsExitShortcut(t *testing.T) {
	cases := []struct {
		width, height int
		m             model
		exit          string
	}{
		{40, 2, model{ready: true, status: &manager.Status{Installed: true, InstanceState: manager.Running}}, "q)"},
		{20, 3, model{ready: true, mode: modeConfirmUninstall, status: &manager.Status{Installed: true}}, "n)"},
		{10, 3, model{ready: true, mode: modeConfig, configTab: configTabPreview}, "q)"},
		{5, 1, model{ready: true, status: &manager.Status{Installed: true, InstanceState: manager.Running}}, "q"},
	}
	for _, tc := range cases {
		m := tc.m
		m.width, m.height = tc.width, tc.height
		view := m.View()
		assertFrame(t, view, tc.width, tc.height)
		flat := strings.ReplaceAll(ansi.Strip(view), "\n", " ")
		if !strings.Contains(flat, tc.exit) {
			t.Fatalf("%dx%d should keep the exit shortcut %q:\n%s", tc.width, tc.height, tc.exit, view)
		}
		if !strings.Contains(flat, "Terminal too small") && tc.width > 10 {
			t.Fatalf("%dx%d should explain the required size:\n%s", tc.width, tc.height, view)
		}
	}
}

// styleClosed reports whether a rendered line resets every style it opened, so
// no colour can leak into the next line of the frame.
func styleClosed(line string) bool {
	lastOpen, lastReset := -1, -1
	for i := 0; i+1 < len(line); {
		if line[i] == 0x1b && line[i+1] == '[' {
			end := strings.IndexByte(line[i+2:], 'm')
			if end < 0 {
				return false
			}
			params := line[i+2 : i+2+end]
			if params == "0" || params == "" {
				lastReset = i
			} else {
				lastOpen = i
			}
			i += 2 + end + 1
			continue
		}
		i++
	}
	return lastOpen <= lastReset
}

func assertStylesClosed(t *testing.T, view string) {
	t.Helper()
	for i, line := range strings.Split(view, "\n") {
		if !styleClosed(line) {
			t.Fatalf("line %d leaves a style open, colours would leak: %q", i, line)
		}
	}
}

func TestTUIStylesCloseOnEveryLine(t *testing.T) {
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })

	details := strings.Repeat("upstream returned an unexpected response at line 4; ", 40) + "ZZ-END-ZZ"
	m := model{
		ready:        true,
		status:       &manager.Status{Installed: true, InstanceState: manager.Running},
		configStatus: manager.ConfigApplyStatus{State: manager.ConfigValidationFailed, ErrorSummary: details},
		width:        60,
		height:       16,
	}
	view := m.View()
	if !strings.Contains(view, "\x1b[") {
		t.Fatalf("truecolor profile should emit escapes:\n%q", view)
	}
	assertStylesClosed(t, view)
	for i := 0; i < 40; i++ {
		updated, _ := m.Update(tuiKey("pgdown"))
		m = updated.(model)
	}
	assertStylesClosed(t, m.View())

	updated, _ := m.Update(tuiKey("c"))
	assertStylesClosed(t, updated.(model).View())

	uninstall := model{ready: true, status: m.status, mode: modeConfirmUninstall, keepBackup: true, width: 60, height: 16}
	assertStylesClosed(t, uninstall.View())

	versions := model{ready: true, status: m.status, mode: modeChooseVersion,
		versions: []manager.VersionInfo{{Tag: "v1.19.0"}, {Tag: "v1.19.1"}}, selectedIdx: 1, width: 60, height: 16}
	assertStylesClosed(t, versions.View())

	schedule := model{ready: true, status: m.status, mode: modeSchedule, scheduleInterval: time.Hour, scheduleActive: true, selectedIdx: 1, width: 60, height: 16}
	assertStylesClosed(t, schedule.View())

	progress := model{ready: true, executing: actUpgrade, phaseLabel: "fetch",
		phaseMsg: strings.Repeat("downloading mihomo ", 50), startedAt: time.Now(), width: 60, height: 16}
	assertStylesClosed(t, progress.View())

	for _, sample := range tuiPageSamples() {
		std := sample.m
		std.width, std.height = 80, 24
		assertStylesClosed(t, std.View())
	}
}

func TestTUIStandardLayoutSeparatorsAndGrouping(t *testing.T) {
	running := &manager.Status{Installed: true, InstanceState: manager.Running, Version: "v1.19.0", AutoStartEnabled: true}
	m := model{
		ready:            true,
		status:           running,
		configStatus:     manager.ConfigApplyStatus{State: manager.ConfigApplied},
		scheduleInterval: time.Hour,
		scheduleActive:   true,
		width:            80,
		height:           24,
	}
	view := ansi.Strip(m.View())
	assertFrame(t, m.View(), 80, 24)

	fullRule := strings.Repeat("─", 80)
	if count := strings.Count(view, fullRule); count != 2 {
		t.Fatalf("standard layout should render header and footer rules, saw %d:\n%s", count, view)
	}
	for _, want := range []string{
		"● running",
		"Core: v1.19.0",
		"Config: applied",
		"Autostart: on",
		"Schedule: every 1h",
		"SERVICE CONTROL",
		"CONFIGURATION",
		"MAINTENANCE",
		"2) Stop",
		"3) Restart",
		"4) Reload",
		"5) Upgrade",
		"▸ c) Config",
		"a) Disable autostart",
		"t) Schedule",
		"u) Uninstall",
		"↑↓/Tab) Focus · Enter) Run · ?) Help · r) Refresh · q) Quit",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("standard dashboard view missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "1) Start") {
		t.Fatalf("running dashboard should hide unavailable Start action:\n%s", view)
	}

	m.width, m.height = 60, 16
	compactView := ansi.Strip(m.View())
	assertFrame(t, m.View(), 60, 16)
	if strings.Contains(compactView, strings.Repeat("─", 60)) {
		t.Fatalf("compact layout should omit full-width section rules to preserve height:\n%s", compactView)
	}
	for _, want := range []string{"2) Stop", "3) Restart", "▸ c) Config", "u) Uninstall"} {
		if !strings.Contains(compactView, want) {
			t.Fatalf("compact layout missing %q:\n%s", want, compactView)
		}
	}
}

func TestTUIDashboardFocusAndHelpNavigation(t *testing.T) {
	ctrl := &tuiMockControl{}
	m := model{
		ready:   true,
		status:  &manager.Status{Installed: true, InstanceState: manager.Running, Version: "v1.19.0"},
		control: ctrl,
		config:  &tuiMockConfig{},
		width:   80,
		height:  24,
	}
	if !strings.Contains(ansi.Strip(m.View()), "▸ c) Config") {
		t.Fatalf("initial focus should be on Config:\n%s", ansi.Strip(m.View()))
	}
	// Deliberately enter the service column, then move from Stop to Restart.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	updated, _ = updated.(model).Update(tuiKey("down"))
	m = updated.(model)
	if m.actionKey != "3" {
		t.Fatalf("focused key after down = %q, want 3", m.actionKey)
	}
	if !strings.Contains(ansi.Strip(m.View()), "▸ 3) Restart") {
		t.Fatalf("expected focus indicator on 3) Restart:\n%s", ansi.Strip(m.View()))
	}

	// Pressing right jumps to the right column (CONFIGURATION / MAINTENANCE).
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = updated.(model)
	if !strings.Contains(ansi.Strip(m.View()), "▸ a) Enable autostart") {
		t.Fatalf("expected right column focus on a) Enable autostart:\n%s", ansi.Strip(m.View()))
	}

	// Pressing left jumps back to the left column ("3) Restart") and Enter dispatches it.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	m = updated.(model)
	updated, cmd := m.Update(tuiKey("enter"))
	m = updated.(model)
	if m.executing != actRestart || cmd == nil {
		t.Fatalf("Enter on focused Restart should start actRestart, got %v", m.executing)
	}

	// Pressing ? toggles the Help view, and q returns to Status.
	m.executing = actNone
	updated, _ = m.Update(tuiKey("?"))
	m = updated.(model)
	if m.mode != modeHelp || !strings.Contains(ansi.Strip(m.View()), "KEYBOARD SHORTCUTS & STATUS GUIDE") {
		t.Fatalf("? should open the Help view:\n%s", ansi.Strip(m.View()))
	}
	updated, _ = m.Update(tuiKey("q"))
	m = updated.(model)
	if m.mode != modeStatus {
		t.Fatalf("q from Help should return to modeStatus, got %v", m.mode)
	}
}

func TestTUISubPagesVisualIndicators(t *testing.T) {
	running := &manager.Status{Installed: true, InstanceState: manager.Running, Version: "v1.19.0"}

	cfg := model{ready: true, status: running, mode: modeConfig, configTab: configTabSubscription, width: 80, height: 24}
	cfgView := ansi.Strip(cfg.View())
	if !strings.Contains(cfgView, "● [Subscription]  ·  ○ Override  ·  ○ Preview") {
		t.Fatalf("config tabs should highlight active tab with indicator:\n%s", cfgView)
	}
	if !strings.Contains(cfgView, "╭") {
		t.Fatalf("standard config subscription tab should render inside a card:\n%s", cfgView)
	}
	if !strings.Contains(cfgView, "Status: unknown (no managed subscription or override applied yet)") {
		t.Fatalf("config subscription tab should explain unknown config state:\n%s", cfgView)
	}

	ver := model{
		ready:       true,
		status:      running,
		mode:        modeChooseVersion,
		versions:    []manager.VersionInfo{{Tag: "v1.19.0"}, {Tag: "v1.19.1"}},
		selectedIdx: 1,
		width:       80,
		height:      24,
	}
	verView := ansi.Strip(ver.View())
	if !strings.Contains(verView, "  v1.19.0 (current)") {
		t.Fatalf("version list should mark the installed version as (current):\n%s", verView)
	}
	if !strings.Contains(verView, "> v1.19.1") {
		t.Fatalf("version list should mark the selected version:\n%s", verView)
	}

	sched := model{
		ready:            true,
		status:           running,
		mode:             modeSchedule,
		scheduleInterval: 6 * time.Hour,
		scheduleActive:   true,
		selectedIdx:      1,
		width:            80,
		height:           24,
	}
	schedView := ansi.Strip(sched.View())
	if !strings.Contains(schedView, "  6h (current)") {
		t.Fatalf("schedule list should mark the active schedule as (current):\n%s", schedView)
	}

	uninst := model{ready: true, status: running, mode: modeConfirmUninstall, width: 80, height: 24}
	uninstView := ansi.Strip(uninst.View())
	if !strings.Contains(uninstView, "b) Toggle backup") {
		t.Fatalf("uninstall dialog should advertise the backup toggle shortcut:\n%s", uninstView)
	}
}

func envWithOverrides(extra map[string]string) []string {
	out := make([]string, 0, len(os.Environ())+len(extra))
	for _, kv := range os.Environ() {
		key := strings.SplitN(kv, "=", 2)[0]
		if _, replaced := extra[key]; !replaced {
			out = append(out, kv)
		}
	}
	for key, value := range extra {
		out = append(out, key+"="+value)
	}
	return out
}

func TestTUINoColorEnvironment(t *testing.T) {
	if os.Getenv("TUI_NO_COLOR_CHILD") == "1" {
		m := model{ready: true, status: &manager.Status{Installed: true, InstanceState: manager.Running}, width: 80, height: 24}
		view := m.View()
		if strings.Contains(view, "\x1b[") {
			t.Fatalf("NO_COLOR should suppress styling, got %q", view)
		}
		return
	}
	command := exec.Command(os.Args[0], "-test.run=^TestTUINoColorEnvironment$")
	command.Env = envWithOverrides(map[string]string{
		"TUI_NO_COLOR_CHILD": "1",
		"NO_COLOR":           "1",
		"CLICOLOR_FORCE":     "1",
		"TERM":               "xterm-256color",
		"COLORTERM":          "truecolor",
	})
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("NO_COLOR child process failed: %v\n%s", err, output)
	}
}
