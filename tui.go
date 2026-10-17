package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/heihei0299/mihomo-manage/internal/manager"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewMode int

const (
	modeStatus viewMode = iota
	modeConfirmUninstall
	modeChooseVersion
	modeConfig
	modeSchedule
)

type configTab int

const (
	configTabSubscription configTab = iota
	configTabOverride
	configTabPreview
)

type action int

const (
	actNone action = iota
	actStart
	actStop
	actRestart
	actReload
	actInstall
	actUpgrade
	actUninstall
	actAutostartOn
	actAutostartOff
)

type actionDef struct {
	enabled func(*manager.Status) bool
}

var (
	installedStopped = func(s *manager.Status) bool { return s != nil && s.Installed && s.InstanceState == manager.Stopped }
	installedRunning = func(s *manager.Status) bool { return s != nil && s.Installed && s.InstanceState == manager.Running }
)

var actionRegistry = map[action]actionDef{
	actStart:   {enabled: installedStopped},
	actStop:    {enabled: installedRunning},
	actRestart: {enabled: installedRunning},
	actReload:  {enabled: installedRunning},
}

// The TUI palette. Lipgloss downgrades to plain text when the output is not a
// terminal or NO_COLOR is set; adaptive colors stay readable on both light and
// dark terminals.
var (
	colRunning = lipgloss.AdaptiveColor{Light: "#15803d", Dark: "#4ade80"}
	colStopped = lipgloss.AdaptiveColor{Light: "#b45309", Dark: "#fbbf24"}
	colError   = lipgloss.AdaptiveColor{Light: "#b91c1c", Dark: "#f87171"}
	colAccent  = lipgloss.AdaptiveColor{Light: "#1d4ed8", Dark: "#60a5fa"}
	colDim     = lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#9ca3af"}
)

var (
	styleRunning   = lipgloss.NewStyle().Foreground(colRunning).Bold(true)
	styleStopped   = lipgloss.NewStyle().Foreground(colStopped).Bold(true)
	styleUpgrading = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styleSuccess   = lipgloss.NewStyle().Foreground(colRunning).Bold(true)
	styleError     = lipgloss.NewStyle().Foreground(colError)
	styleAccent    = lipgloss.NewStyle().Foreground(colAccent)
	styleSelected  = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styleDim       = lipgloss.NewStyle().Foreground(colDim)
	styleCard      = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colDim).
			Padding(0, 1)
	styleDialog = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colDim).
			Padding(1, 2)
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const spinnerInterval = 80 * time.Millisecond

type tickMsg struct{}

func tickCmd() tea.Cmd {
	return tea.Tick(spinnerInterval, func(time.Time) tea.Msg { return tickMsg{} })
}

type model struct {
	ctx              context.Context
	control          manager.ServiceControl
	lifecycle        manager.LifecycleManager
	config           manager.ConfigManager
	schedule         manager.ScheduleManager
	status           *manager.Status
	statusErr        error
	configStatus     manager.ConfigApplyStatus
	configErr        error
	scheduleInterval time.Duration
	scheduleActive   bool
	scheduleErr      error
	ready            bool
	executing        action
	execResult       string
	actionErr        error
	phaseLabel       string
	phaseMsg         string
	startedAt        time.Time
	spinnerFrame     int

	width  int
	height int

	mode           viewMode
	keepBackup     bool
	versions       []manager.VersionInfo
	versionsErr    error
	selectedIdx    int
	configTab      configTab
	previewContent string
	previewOffset  int
	previewErr     error
	previewLoading bool
}

type statusMsg struct {
	status           *manager.Status
	err              error
	configStatus     manager.ConfigApplyStatus
	configErr        error
	scheduleInterval time.Duration
	scheduleActive   bool
	scheduleErr      error
}

type actionDoneMsg struct {
	action action
	err    error
}

type progressMsg struct {
	phase   manager.InstallationPhase
	message string
	err     error
	nextCmd tea.Cmd
}

type versionsMsg struct {
	versions []manager.VersionInfo
	err      error
}

type configPreviewMsg struct {
	content string
	err     error
}

type subscriptionEditMsg struct {
	err error
}

type scheduleDoneMsg struct {
	interval time.Duration
	err      error
}

var schedulePresets = []time.Duration{0, time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour}

func editSubscriptionCmd(cfg manager.ConfigManager, ctx context.Context) tea.Cmd {
	file, err := os.CreateTemp("", "mihomo-subscription-*")
	if err != nil {
		return func() tea.Msg { return subscriptionEditMsg{err: err} }
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		os.Remove(path)
		return func() tea.Msg { return subscriptionEditMsg{err: err} }
	}

	command, err := configuredEditorCommand(path)
	if err != nil {
		os.Remove(path)
		return func() tea.Msg { return subscriptionEditMsg{err: err} }
	}
	return tea.ExecProcess(command, func(runErr error) tea.Msg {
		defer os.Remove(path)
		if runErr != nil {
			return subscriptionEditMsg{err: fmt.Errorf("editor failed: %w", runErr)}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return subscriptionEditMsg{err: err}
		}
		if strings.TrimSpace(string(data)) == "" {
			return subscriptionEditMsg{err: fmt.Errorf("subscription source cannot be empty")}
		}
		if err := cfg.SetSubscriptionSource(ctx, string(data)); err != nil {
			return subscriptionEditMsg{err: err}
		}
		return subscriptionEditMsg{}
	})
}

func tuiContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func (m model) Init() tea.Cmd {
	return fetchStatusCmd(m.control, m.config, m.schedule, tuiContext(m.ctx))
}

func fetchStatusCmd(ctrl manager.ServiceControl, cfg manager.ConfigManager, schedule manager.ScheduleManager, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		s, err := ctrl.Status(ctx)
		configStatus, configErr := cfg.LastConfigApply(ctx)
		var interval time.Duration
		var active bool
		var scheduleErr error
		if schedule != nil {
			interval, active, scheduleErr = schedule.ScheduleStatus(ctx)
		}
		return statusMsg{status: s, err: err, configStatus: configStatus, configErr: configErr, scheduleInterval: interval, scheduleActive: active, scheduleErr: scheduleErr}
	}
}

func fetchConfigPreview(cfg manager.ConfigManager, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		s, err := cfg.PreviewConfig(ctx)
		return configPreviewMsg{content: s, err: err}
	}
}

func fetchVersionsCmd(lifecycle manager.LifecycleManager, ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		v, err := lifecycle.ListVersions(ctx)
		return versionsMsg{versions: v, err: err}
	}
}

func execActionCmd(ctrl manager.ServiceControl, lifecycle manager.LifecycleManager, a action, progressCh chan<- progressMsg, version string, keepBackup bool) tea.Cmd {
	return execActionCmdWithContext(context.Background(), ctrl, lifecycle, a, progressCh, version, keepBackup)
}

func execActionCmdWithContext(ctx context.Context, ctrl manager.ServiceControl, lifecycle manager.LifecycleManager, a action, progressCh chan<- progressMsg, version string, keepBackup bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch a {
		case actStart:
			err = ctrl.Start(ctx)
		case actStop:
			err = ctrl.Stop(ctx)
		case actRestart:
			err = ctrl.Restart(ctx)
		case actReload:
			err = ctrl.Reload(ctx)
		case actInstall:
			err = lifecycle.Install(ctx, version, true, func(e manager.ProgressEvent) {
				if progressCh != nil {
					progressCh <- progressMsg{phase: e.Phase, message: e.Message, err: e.Error}
				}
			})
		case actUpgrade:
			err = lifecycle.Upgrade(ctx, version, func(e manager.ProgressEvent) {
				if progressCh != nil {
					progressCh <- progressMsg{phase: e.Phase, message: e.Message, err: e.Error}
				}
			})
		case actUninstall:
			err = lifecycle.Uninstall(ctx, keepBackup, func(e manager.ProgressEvent) {
				if progressCh != nil {
					progressCh <- progressMsg{phase: e.Phase, message: e.Message, err: e.Error}
				}
			})
		case actAutostartOn:
			err = ctrl.SetAutoStart(ctx, true)
		case actAutostartOff:
			err = ctrl.SetAutoStart(ctx, false)
		}
		if progressCh != nil {
			close(progressCh)
		}
		return actionDoneMsg{action: a, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if max := m.previewMaxOffset(); m.previewOffset > max {
			m.previewOffset = max
		}
		return m, nil

	case tickMsg:
		if m.executing == actNone {
			return m, nil
		}
		m.spinnerFrame++
		return m, tickCmd()

	case tea.KeyMsg:
		if m.executing != actNone {
			return m, nil
		}

		switch m.mode {
		case modeConfirmUninstall:
			return m.updateConfirmUninstall(msg)
		case modeChooseVersion:
			return m.updateChooseVersion(msg)
		case modeConfig:
			return m.updateConfigMode(msg)
		case modeSchedule:
			return m.updateScheduleMode(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "c":
			if isInstalled(m.status) {
				m.mode = modeConfig
				m.configTab = configTabSubscription
				m.execResult = ""
				m.actionErr = nil
				m.previewErr = nil
				m.previewOffset = 0
				m.previewLoading = true
				return m, fetchConfigPreview(m.config, tuiContext(m.ctx))
			}
		case "r":
			return m, fetchStatusCmd(m.control, m.config, m.schedule, tuiContext(m.ctx))
		case "1":
			if isActionAllowed(m.status, actStart) {
				return m.startAction(actStart, "latest")
			}
		case "2":
			if isActionAllowed(m.status, actStop) {
				return m.startAction(actStop, "")
			}
		case "3":
			if isActionAllowed(m.status, actRestart) {
				return m.startAction(actRestart, "")
			}
		case "4":
			if isActionAllowed(m.status, actReload) {
				return m.startAction(actReload, "")
			}
		case "5":
			if m.status != nil && m.status.Installed {
				m.mode = modeChooseVersion
				m.versions = nil
				m.versionsErr = nil
				m.selectedIdx = 0
				return m, fetchVersionsCmd(m.lifecycle, tuiContext(m.ctx))
			}
		case "i":
			if !isInstalled(m.status) {
				return m.startAction(actInstall, "latest")
			}
		case "a":
			if isInstalled(m.status) {
				if m.status.AutoStartEnabled {
					return m.startAction(actAutostartOff, "")
				}
				return m.startAction(actAutostartOn, "")
			}
		case "t":
			if isInstalled(m.status) {
				m.mode = modeSchedule
				m.selectedIdx = schedulePresetIndex(m.scheduleInterval, m.scheduleActive)
				return m, nil
			}
		case "u":
			if isInstalled(m.status) {
				m.mode = modeConfirmUninstall
				m.keepBackup = false
				return m, nil
			}
		}
		return m, nil

	case statusMsg:
		m.ready = true
		m.status = msg.status
		m.statusErr = msg.err
		m.configStatus = msg.configStatus
		m.configErr = msg.configErr
		m.scheduleInterval = msg.scheduleInterval
		m.scheduleActive = msg.scheduleActive
		m.scheduleErr = msg.scheduleErr
		return m, nil

	case versionsMsg:
		m.versionsErr = msg.err
		if msg.err == nil {
			m.versions = msg.versions
		} else {
			m.versions = nil
		}
		return m, nil

	case configPreviewMsg:
		m.previewLoading = false
		m.previewErr = msg.err
		if msg.err == nil {
			m.previewContent = msg.content
			m.previewOffset = 0
		}
		return m, nil

	case scheduleDoneMsg:
		m.mode = modeStatus
		m.execResult = ""
		m.actionErr = msg.err
		if msg.err != nil {
			m.execResult = "failed"
		} else {
			m.execResult = "success"
		}
		return m, fetchStatusCmd(m.control, m.config, m.schedule, tuiContext(m.ctx))

	case subscriptionEditMsg:
		m.execResult = ""
		m.actionErr = msg.err
		if msg.err != nil {
			m.execResult = "failed"
		} else {
			m.execResult = "success"
		}
		m.previewLoading = true
		m.previewErr = nil
		return m, fetchConfigPreview(m.config, tuiContext(m.ctx))

	case progressMsg:
		m.phaseLabel = msg.phase.String()
		m.phaseMsg = msg.message
		return m, msg.nextCmd

	case actionDoneMsg:
		m.executing = actNone
		m.mode = modeStatus
		if msg.err != nil {
			m.execResult = "failed"
			m.actionErr = msg.err
		} else {
			m.execResult = "success"
			m.actionErr = nil
		}
		return m, fetchStatusCmd(m.control, m.config, m.schedule, tuiContext(m.ctx))
	}
	return m, nil
}

func (m model) updateConfirmUninstall(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		return m.startAction(actUninstall, "")
	case "n", "N", "q":
		m.mode = modeStatus
		return m, nil
	case "b", "B":
		m.keepBackup = !m.keepBackup
		return m, nil
	}
	return m, nil
}

func (m model) updateChooseVersion(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "down", "j":
		if m.selectedIdx < len(m.versions)-1 {
			m.selectedIdx++
		}
		return m, nil
	case "up", "k":
		if m.selectedIdx > 0 {
			m.selectedIdx--
		}
		return m, nil
	case "enter":
		if len(m.versions) > 0 {
			ver := m.versions[m.selectedIdx]
			return m.startAction(actUpgrade, ver.Tag)
		}
		return m, nil
	case "q", "esc":
		m.mode = modeStatus
		return m, nil
	}
	return m, nil
}

func schedulePresetIndex(interval time.Duration, active bool) int {
	if !active {
		return 0
	}
	for i, preset := range schedulePresets {
		if preset == interval {
			return i
		}
	}
	return 0
}

func scheduleCommand(sched manager.ScheduleManager, ctx context.Context, interval time.Duration) tea.Cmd {
	return func() tea.Msg {
		var err error
		if interval == 0 {
			err = sched.StopSchedule(ctx)
		} else {
			err = sched.SetSchedule(ctx, interval)
		}
		return scheduleDoneMsg{interval: interval, err: err}
	}
}

func (m model) updateScheduleMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.mode = modeStatus
		return m, nil
	case "up", "k":
		if m.selectedIdx > 0 {
			m.selectedIdx--
		}
	case "down", "j":
		if m.selectedIdx < len(schedulePresets)-1 {
			m.selectedIdx++
		}
	case "enter":
		return m, scheduleCommand(m.schedule, tuiContext(m.ctx), schedulePresets[m.selectedIdx])
	}
	return m, nil
}

func (m model) updateConfigMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.mode = modeStatus
		return m, nil
	case "tab", "right":
		m.configTab = (m.configTab + 1) % 3
		return m, nil
	case "left":
		m.configTab = (m.configTab - 1 + 3) % 3
		return m, nil
	case "r":
		m.previewLoading = true
		m.previewErr = nil
		return m, fetchConfigPreview(m.config, tuiContext(m.ctx))
	case "e":
		if m.configTab == configTabSubscription {
			return m, editSubscriptionCmd(m.config, tuiContext(m.ctx))
		}
	case "up", "k", "down", "j", "pgup", "pgdown":
		m.scrollPreview(msg.String())
	}
	return m, nil
}

func (m *model) scrollPreview(key string) {
	if m.configTab != configTabPreview {
		return
	}
	max := m.previewMaxOffset()
	switch key {
	case "up", "k":
		m.previewOffset--
	case "down", "j":
		m.previewOffset++
	case "pgup":
		m.previewOffset -= m.previewHeight()
	case "pgdown":
		m.previewOffset += m.previewHeight()
	}
	if m.previewOffset < 0 {
		m.previewOffset = 0
	}
	if m.previewOffset > max {
		m.previewOffset = max
	}
}

func (m model) startAction(a action, version string) (tea.Model, tea.Cmd) {
	m.executing = a
	m.mode = modeStatus
	m.startedAt = time.Now()
	m.spinnerFrame = 0
	m.phaseLabel = ""
	m.phaseMsg = ""
	ch := make(chan progressMsg, 20)
	return m, tea.Batch(
		tickCmd(),
		progressReaderCmd(ch),
		execActionCmdWithContext(tuiContext(m.ctx), m.control, m.lifecycle, a, ch, version, m.keepBackup),
	)
}

func progressReaderCmd(ch <-chan progressMsg) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return nil
		}
		e.nextCmd = progressReaderCmd(ch)
		return e
	}
}

func isInstalled(s *manager.Status) bool {
	return s != nil && s.Installed
}

func isActionAllowed(s *manager.Status, a action) bool {
	if s == nil {
		return false
	}
	if !s.Installed {
		return a == actInstall
	}
	if def, ok := actionRegistry[a]; ok {
		return def.enabled(s)
	}
	return false
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "off"
	}
	var b strings.Builder
	if h := d / time.Hour; h > 0 {
		fmt.Fprintf(&b, "%dh", h)
	}
	if m := (d % time.Hour) / time.Minute; m > 0 {
		fmt.Fprintf(&b, "%dm", m)
	}
	if s := (d % time.Minute) / time.Second; s > 0 {
		fmt.Fprintf(&b, "%ds", s)
	}
	return b.String()
}

func scheduleSummary(err error, active bool, interval time.Duration) string {
	if err != nil {
		var legacy manager.LegacyScheduleError
		if errors.As(err, &legacy) {
			return "legacy"
		}
		return "unknown"
	}
	if active {
		return "every " + formatDuration(interval)
	}
	return "off"
}

func helpLine(items ...string) string {
	return styleDim.Render(strings.Join(items, "   "))
}

func joinColumns(left, right []string, gap string) string {
	width := 0
	for _, cell := range left {
		if w := lipgloss.Width(cell); w > width {
			width = w
		}
	}
	rows := make([]string, 0, len(left))
	for i, cell := range left {
		row := cell + strings.Repeat(" ", width-lipgloss.Width(cell))
		if i < len(right) && right[i] != "" {
			row += gap + right[i]
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}

func renderActionToken(key, label string, enabled bool) string {
	token := key + ") " + label
	if !enabled {
		return styleDim.Render(token)
	}
	return token
}

func statusActions(s *manager.Status) string {
	if !s.Installed {
		return renderActionToken("i", "Install", true)
	}
	autostart := "Autostart on"
	if s.AutoStartEnabled {
		autostart = "Autostart off"
	}
	row1 := strings.Join([]string{
		renderActionToken("1", "Start", isActionAllowed(s, actStart)),
		renderActionToken("2", "Stop", isActionAllowed(s, actStop)),
		renderActionToken("3", "Restart", isActionAllowed(s, actRestart)),
		renderActionToken("4", "Reload", isActionAllowed(s, actReload)),
		renderActionToken("5", "Upgrade", true),
	}, "  ")
	row2 := strings.Join([]string{
		renderActionToken("c", "Config", true),
		renderActionToken("a", autostart, true),
		renderActionToken("t", "Schedule", true),
		renderActionToken("u", "Uninstall", true),
	}, "  ")
	return row1 + "\n" + row2
}

// windowLines returns the visible slice of lines and the offset clamped to a
// valid window position.
func windowLines(lines []string, offset, height int) ([]string, int) {
	if height <= 0 || len(lines) == 0 {
		return nil, 0
	}
	max := len(lines) - height
	if max < 0 {
		max = 0
	}
	if offset > max {
		offset = max
	}
	if offset < 0 {
		offset = 0
	}
	end := offset + height
	if end > len(lines) {
		end = len(lines)
	}
	return lines[offset:end], offset
}

func (m model) previewHeight() int {
	h := m.height - 5
	if h < 3 {
		return 3
	}
	return h
}

func (m model) previewMaxOffset() int {
	lines := strings.Count(m.previewContent, "\n") + 1
	max := lines - m.previewHeight()
	if max < 0 {
		return 0
	}
	return max
}

func (m model) View() string {
	if !m.ready {
		return "Checking mihomo status...\n\n" + helpLine("q) Quit")
	}
	if m.statusErr != nil {
		return styleError.Render(fmt.Sprintf("Error: %v", m.statusErr)) + "\n\n" + helpLine("r) Retry", "q) Quit")
	}

	if m.executing != actNone {
		return m.executingView()
	}

	switch m.mode {
	case modeConfirmUninstall:
		return m.uninstallView()
	case modeChooseVersion:
		return m.versionChoiceView()
	case modeConfig:
		return m.configView()
	case modeSchedule:
		return m.scheduleView()
	}

	return m.statusView()
}

func (m model) executingView() string {
	label := "working..."
	switch m.executing {
	case actStart:
		label = "starting..."
	case actStop:
		label = "stopping..."
	case actRestart:
		label = "restarting..."
	case actReload:
		label = "reloading..."
	case actInstall:
		label = "installing..."
	case actUpgrade:
		label = "upgrading..."
	case actUninstall:
		label = "uninstalling..."
	case actAutostartOn:
		label = "enabling autostart..."
	case actAutostartOff:
		label = "disabling autostart..."
	}
	frame := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
	lines := []string{styleAccent.Render(label + " " + frame)}
	if m.phaseMsg != "" {
		lines = append(lines, styleDim.Render(fmt.Sprintf("[%s] %s", m.phaseLabel, m.phaseMsg)))
	}
	if !m.startedAt.IsZero() {
		lines = append(lines, styleDim.Render("elapsed "+time.Since(m.startedAt).Round(time.Second).String()))
	}
	return strings.Join(lines, "\n")
}

func (m model) uninstallView() string {
	backup := "no"
	if m.keepBackup {
		backup = "yes"
	}
	body := fmt.Sprintf("Uninstall mihomo?\n\nbackup: %s  (b to toggle)", backup)
	return styleDialog.Render(body) + "\n\n" + helpLine("y) Confirm", "n) Cancel")
}

func (m model) versionChoiceView() string {
	header := "Select version"
	if m.status != nil && m.status.Version != "" {
		header += "\n\ninstalled: " + m.status.Version
	}
	footer := helpLine("↑↓) select", "Enter) confirm", "q) cancel")

	if m.versionsErr != nil {
		return header + "\n\n" + styleError.Render("Error loading versions: "+m.versionsErr.Error()) + "\n\n" + footer
	}
	if len(m.versions) == 0 {
		return header + "\n\nNo releases available.\n\n" + footer
	}
	lines := make([]string, 0, len(m.versions))
	for i, v := range m.versions {
		if i == m.selectedIdx {
			lines = append(lines, styleSelected.Render("> "+v.Tag))
		} else {
			lines = append(lines, "  "+v.Tag)
		}
	}
	return header + "\n\n" + strings.Join(lines, "\n") + "\n\n" + footer
}

func (m model) statusView() string {
	s := m.status
	if s == nil {
		return "No status available.\n\n" + helpLine("r) Refresh", "q) Quit")
	}

	version := s.Version
	if version == "" {
		version = "unknown"
	}

	autostart := "off"
	if s.AutoStartEnabled {
		autostart = "on"
	}

	state := s.InstanceState.String()
	glyph := "●"
	stateStyle := styleStopped
	switch {
	case !s.Installed:
		state = "not installed"
		glyph = "○"
		stateStyle = styleDim
	case s.InstanceState == manager.Running:
		stateStyle = styleRunning
	case s.InstanceState == manager.Upgrading:
		glyph = "◐"
		stateStyle = styleUpgrading
	}
	stateCell := stateStyle.Render(glyph + " " + state)

	configState := string(m.configStatus.State)
	if configState == "" || m.configErr != nil {
		configState = "unknown"
	}
	configCell := "config: " + configState
	switch {
	case m.configErr != nil:
		configCell = styleDim.Render(configCell)
	case m.configStatus.State == manager.ConfigApplyFailed ||
		m.configStatus.State == manager.ConfigValidationFailed ||
		m.configStatus.ErrorSummary != "":
		configCell = styleError.Render(configCell)
	}

	scheduleText := scheduleSummary(m.scheduleErr, m.scheduleActive, m.scheduleInterval)
	scheduleCell := "schedule: " + scheduleText
	if scheduleText == "legacy" || scheduleText == "unknown" {
		scheduleCell = styleStopped.Render(scheduleCell)
	}

	var body string
	var narrow []string
	if !s.Installed {
		body = stateCell
		narrow = []string{stateCell}
	} else {
		body = joinColumns(
			[]string{stateCell, "autostart: " + autostart, scheduleCell},
			[]string{"version: " + version, configCell, ""},
			"   ",
		)
		narrow = []string{stateCell, "version: " + version, "autostart: " + autostart, configCell, scheduleCell}
	}
	card := styleCard.Render(body)
	if m.width > 0 && lipgloss.Width(card) > m.width {
		card = strings.Join(narrow, "\n")
	}

	sections := []string{card}
	if m.configStatus.ErrorSummary != "" {
		sections = append(sections, styleError.Render("Config error: "+m.configStatus.ErrorSummary))
	}
	if m.execResult == "success" {
		sections = append(sections, styleSuccess.Render("✓ Operation succeeded"))
	} else if m.execResult == "failed" {
		sections = append(sections, styleError.Render(fmt.Sprintf("✗ %v", m.actionErr)))
	}
	sections = append(sections, "", statusActions(s), "", helpLine("r) Refresh", "q) Quit"))
	return strings.Join(sections, "\n")
}

func (m model) configView() string {
	tabs := []string{"Subscription", "Override", "Preview"}
	tabParts := make([]string, 0, len(tabs))
	for i, t := range tabs {
		if configTab(i) == m.configTab {
			tabParts = append(tabParts, styleSelected.Render("["+t+"]"))
		} else {
			tabParts = append(tabParts, styleDim.Render(" "+t+" "))
		}
	}
	tabLine := strings.Join(tabParts, " ")

	var content, footer string
	switch m.configTab {
	case configTabSubscription:
		content = "Subscription source: remote or local\n" +
			styleDim.Render("e) Edit source with $EDITOR (URL or local subscription-data)") + "\n" +
			styleDim.Render("CLI: mihomo-manager subscription set <url-or-data>")
		footer = helpLine("Tab/← →) switch tab", "e) Edit", "r) Refresh preview", "q) Back")
	case configTabOverride:
		content = fmt.Sprintf("Override file: %s\n", manager.OverrideFilePath) +
			styleDim.Render("Edit with: mihomo-manager config override edit") + "\n\n" +
			"Rules are embedded in the override file's 'rules:' field."
		footer = helpLine("Tab/← →) switch tab", "q) Back")
	case configTabPreview:
		content, footer = m.previewTab()
	}

	result := ""
	if m.execResult == "success" {
		result = "\n" + styleSuccess.Render("✓ Configuration updated")
	} else if m.execResult == "failed" {
		result = "\n" + styleError.Render(fmt.Sprintf("✗ %v", m.actionErr))
	}

	return tabLine + "\n\n" + content + result + "\n\n" + footer
}

func (m model) previewTab() (string, string) {
	switch {
	case m.previewErr != nil:
		return styleError.Render("✗ " + m.previewErr.Error()),
			helpLine("r) Retry", "q) Back")
	case m.previewContent == "" && m.previewLoading:
		return styleDim.Render("Loading preview..."), helpLine("q) Back")
	case m.previewContent == "":
		return styleDim.Render("(empty)"), helpLine("r) Refresh", "q) Back")
	}
	lines := strings.Split(m.previewContent, "\n")
	visible, offset := windowLines(lines, m.previewOffset, m.previewHeight())
	position := fmt.Sprintf("lines %d-%d / %d", offset+1, offset+len(visible), len(lines))
	return strings.Join(visible, "\n"),
		helpLine(position, "↑↓/jk) scroll", "PgUp/PgDn) page", "r) Refresh", "q) Back")
}

func (m model) scheduleView() string {
	lines := make([]string, 0, len(schedulePresets))
	for i, preset := range schedulePresets {
		label := formatDuration(preset)
		if i == m.selectedIdx {
			lines = append(lines, styleSelected.Render("> "+label))
		} else {
			lines = append(lines, "  "+label)
		}
	}
	current := scheduleSummary(m.scheduleErr, m.scheduleActive, m.scheduleInterval)
	return "Schedule subscription update\n\ncurrent: " + current + "\n\n" +
		strings.Join(lines, "\n") + "\n\n" +
		helpLine("↑↓) select", "Enter) apply", "q) cancel")
}

func startTUI(ctx context.Context, ctrl manager.ServiceControl, lifecycle manager.LifecycleManager, cfg manager.ConfigManager, schedule manager.ScheduleManager) error {
	p := tea.NewProgram(model{ctx: ctx, control: ctrl, lifecycle: lifecycle, config: cfg, schedule: schedule}, tea.WithContext(ctx), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
