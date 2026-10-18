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
	"github.com/charmbracelet/x/ansi"
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
	styleTitle     = lipgloss.NewStyle().Bold(true)
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
	readOffset     int
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
		previousWidth := m.viewWidth()
		m.width = msg.Width
		m.height = msg.Height
		if previousWidth != m.viewWidth() && m.previewOffset > 0 && m.currentPage().preview {
			// Re-wrap the preview around the source line that was at the top.
			m.previewOffset = m.previewOffsetForWidth(previousWidth)
		}
		m.clampViewOffset()
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
		if m.layoutMode() == layoutTooSmall {
			return m.updateResizeOnly(msg)
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
				m.resetViewOffsets()
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
				m.resetViewOffsets()
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
				m.resetViewOffsets()
				return m, nil
			}
		case "u":
			if isInstalled(m.status) {
				m.mode = modeConfirmUninstall
				m.keepBackup = false
				m.resetViewOffsets()
				return m, nil
			}
		case "up", "k", "down", "j", "pgup", "pgdown":
			m.scrollBody(msg.String())
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
		// Refreshed status replaces the page content; reading starts over.
		m.readOffset = 0
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
		m.resetViewOffsets()
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
		m.resetViewOffsets()
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
		m.resetViewOffsets()
		return m, nil
	case "b", "B":
		m.keepBackup = !m.keepBackup
		return m, nil
	}
	return m, nil
}

func (m model) updateChooseVersion(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.versionsErr != nil {
		// The version list is replaced by a scrollable error report.
		switch msg.String() {
		case "up", "k", "down", "j", "pgup", "pgdown":
			m.scrollBody(msg.String())
			return m, nil
		}
	}
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
		m.resetViewOffsets()
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
		m.resetViewOffsets()
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
		m.resetViewOffsets()
		return m, nil
	case "tab", "right":
		m.configTab = (m.configTab + 1) % 3
		m.resetViewOffsets()
		return m, nil
	case "left":
		m.configTab = (m.configTab - 1 + 3) % 3
		m.resetViewOffsets()
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
		m.scrollBody(msg.String())
	}
	return m, nil
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

// ---------------------------------------------------------------------------
// Layout
// ---------------------------------------------------------------------------

// Layout thresholds. The dashboard renders a standard layout on terminals of
// at least 80x24, a compact layout down to 60x16 and a resize hint below that.
const (
	standardLayoutWidth  = 80
	standardLayoutHeight = 24
	minimumLayoutWidth   = 60
	minimumLayoutHeight  = 16
	defaultLayoutWidth   = 80
	defaultLayoutHeight  = 24
)

const (
	titleStatus    = "mihomo-manager · Status"
	titleConfig    = "mihomo-manager · Configuration"
	titleVersions  = "mihomo-manager · Select version"
	titleSchedule  = "mihomo-manager · Schedule subscription update"
	titleUninstall = "mihomo-manager · Uninstall"
)

type layoutMode int

const (
	layoutStandard layoutMode = iota
	layoutCompact
	layoutTooSmall
)

// viewWidth is the rendering budget for the current terminal width. Until the
// first window size event arrives the conventional 80x24 budget is used, so
// the dashboard never flashes the resize hint on startup.
func (m model) viewWidth() int {
	if m.width > 0 {
		return m.width
	}
	return defaultLayoutWidth
}

func (m model) viewHeight() int {
	if m.height > 0 {
		return m.height
	}
	return defaultLayoutHeight
}

func (m model) layoutMode() layoutMode {
	w, h := m.viewWidth(), m.viewHeight()
	switch {
	case w >= standardLayoutWidth && h >= standardLayoutHeight:
		return layoutStandard
	case w >= minimumLayoutWidth && h >= minimumLayoutHeight:
		return layoutCompact
	default:
		return layoutTooSmall
	}
}

func clampInt(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func displayWidth(line string) int {
	return ansi.StringWidth(line)
}

// truncateLine shortens a rendered line to width cells and marks the cut with
// an ellipsis so clipped text stays recognisable.
func truncateLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if displayWidth(line) <= width {
		return line
	}
	return ansi.Truncate(line, width, "…")
}

// wrapText lays one logical line out over as many width-cell lines as needed.
// It is a hard wrap: every character survives, including leading indentation,
// runs of spaces and trailing spaces, so configuration text can be rebuilt from
// the screen lines. Tabs are expanded for display only and widths are measured
// in terminal cells (wide runes, combining marks and ANSI styles included). The
// source text itself is never modified.
func wrapText(line string, width int) []string {
	if width < 1 {
		width = 1
	}
	line = strings.ReplaceAll(line, "\t", "    ")
	if displayWidth(line) <= width {
		return []string{line}
	}
	parts := strings.Split(ansi.Hardwrap(line, width, true), "\n")
	if len(parts) == 0 {
		return []string{""}
	}
	return parts
}

// styleBlock wraps every logical line before styling it line by line, so each
// rendered screen line opens and closes its own escape sequences and clipping
// a line cannot lose a colour reset.
func styleBlock(text string, width int, style lipgloss.Style) []string {
	logical := strings.Split(text, "\n")
	lines := make([]string, 0, len(logical))
	for _, line := range logical {
		for _, part := range wrapText(line, width) {
			lines = append(lines, style.Render(part))
		}
	}
	return lines
}

// firstLine returns the first line of text, for the one-line hint area.
func firstLine(text string) string {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return text[:i]
	}
	return text
}

// layoutTokens arranges complete "key) action" tokens into as many lines as the
// width requires. Tokens are never split between lines.
func layoutTokens(tokens []string, width int) []string {
	if width <= 0 || len(tokens) == 0 {
		return nil
	}
	const gap = "   "
	lines := make([]string, 0, 2)
	current := ""
	currentWidth := 0
	for _, token := range tokens {
		if token == "" {
			continue
		}
		token = truncateLine(token, width)
		tokenWidth := displayWidth(token)
		switch {
		case currentWidth == 0:
			current, currentWidth = token, tokenWidth
		case currentWidth+len(gap)+tokenWidth > width:
			lines = append(lines, current)
			current, currentWidth = token, tokenWidth
		default:
			current += gap + token
			currentWidth += len(gap) + tokenWidth
		}
	}
	if currentWidth > 0 {
		lines = append(lines, current)
	}
	return lines
}

// shrinkHelp keeps the shortcut block inside limit lines and always keeps the
// page's exit shortcut visible.
func shrinkHelp(lines []string, limit int, exit string, width int) []string {
	if limit < 1 {
		return nil
	}
	if len(lines) <= limit {
		return lines
	}
	kept := append([]string{}, lines[:limit-1]...)
	if exit == "" {
		return kept
	}
	return append(kept, truncateLine(exit, width))
}

// clipLines trims text to limit lines and marks the omitted remainder.
func clipLines(lines []string, limit, width int) []string {
	if limit <= 0 {
		return nil
	}
	if len(lines) <= limit {
		return lines
	}
	kept := append([]string{}, lines[:limit-1]...)
	return append(kept, truncateLine(lines[limit-1]+"…", width))
}

func styleLines(lines []string, style lipgloss.Style) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = style.Render(line)
	}
	return out
}

func joinLines(lines []string, width int) string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = truncateLine(line, width)
	}
	return strings.Join(out, "\n")
}

// ---------------------------------------------------------------------------
// Page frame
// ---------------------------------------------------------------------------

// page describes one dashboard screen. The shared frame paints a title, an
// optional tab bar, a body, a feedback area and the shortcut lines; only the
// body absorbs whatever height is left over.
type page struct {
	title      string
	tabs       string
	summary    string
	body       []string
	tail       []string
	list       *listSection
	tokens     []string
	exit       []string
	scrollable bool
	offset     int
	origin     []int
	preview    bool
}

// listSection is a selectable list whose window follows the selection so the
// highlighted entry always stays visible.
type listSection struct {
	prefix   []string
	items    [][]string
	selected int
}

// frameInfo reports the body window that was rendered so the update path can
// clamp reading offsets against the same budget. preview marks a body that is
// the wrapped configuration, which is the only body scrolled by previewOffset.
type frameInfo struct {
	valid      bool
	bodyHeight int
	offset     int
	total      int
	preview    bool
	list       bool
}

// frameParts is the page content after the height budget has been applied.
type frameParts struct {
	header   []string
	body     []string
	tail     []string
	position string
	summary  []string
	help     []string
	offset   int
	total    int
	preview  bool
}

var scrollTokens = []string{"↑↓/jk) scroll", "PgUp/PgDn) page"}

func firstToken(tokens []string) string {
	if len(tokens) == 0 {
		return ""
	}
	return tokens[0]
}

// renderPage lays a page out inside the current terminal size.
func (m model) renderPage(p page) (string, frameInfo) {
	width, height := m.viewWidth(), m.viewHeight()
	if m.layoutMode() == layoutTooSmall {
		return renderResizeHint(p, width, height), frameInfo{}
	}
	parts := m.layoutPage(p, width, height)
	lines := make([]string, 0, len(parts.header)+len(parts.body)+len(parts.tail)+len(parts.summary)+len(parts.help)+1)
	lines = append(lines, parts.header...)
	lines = append(lines, parts.body...)
	if len(parts.body) > 0 && len(parts.body) < parts.total && !p.scrollable && p.list == nil {
		// A clipped body on a non-scrollable page (the progress view) ends with
		// an ellipsis instead of silently disappearing.
		lines[len(lines)-1] = truncateLine(lines[len(lines)-1]+"…", width)
	}
	bodyEnd := len(lines)
	if parts.position != "" {
		lines = append(lines, parts.position)
	}
	lines = append(lines, parts.tail...)
	lines = append(lines, parts.summary...)
	lines = append(lines, parts.help...)
	if overflow := len(lines) - height; overflow > 0 {
		// Defensive: the height budget above should already fit, but never paint
		// outside the window. Body lines are dropped before status lines.
		body := lines[len(parts.header):bodyEnd]
		if overflow <= len(body) {
			body = body[:len(body)-overflow]
			tail := append([]string{}, lines[bodyEnd:]...)
			lines = append(append(lines[:len(parts.header)], body...), tail...)
		} else {
			lines = lines[len(lines)-height:]
		}
	}
	return joinLines(lines, width), frameInfo{
		valid:      true,
		bodyHeight: len(parts.body),
		offset:     parts.offset,
		total:      parts.total,
		preview:    parts.preview,
		list:       p.list != nil,
	}
}

// layoutPage resolves the height budget. The header, the pinned tail, the
// one-line summary and the source-range line are measured first and the body
// receives the remainder; the shortcut block is measured once more when a
// scrolling hint is needed. There is no retry loop, so the body window, the
// reading offset and the footer always come from the same budget.
func (m model) layoutPage(p page, width, height int) frameParts {
	parts := frameParts{preview: p.preview, total: contentTotal(p)}
	if p.title != "" {
		parts.header = append(parts.header, truncateLine(styleTitle.Render(p.title), width))
	}
	if p.tabs != "" {
		parts.header = append(parts.header, truncateLine(p.tabs, width))
	}

	pinned := len(p.tail)
	summaryLines := 0
	if p.summary != "" {
		summaryLines = 1
	}
	positionLines := 0
	if p.preview {
		positionLines = 1
	}

	base := append([]string{}, p.tokens...)
	help := layoutTokens(base, width)
	budget := func(helpLines []string) int {
		return height - len(parts.header) - len(helpLines) - summaryLines - positionLines - pinned
	}
	bodyHeight := budget(help)
	if p.scrollable && parts.total > bodyHeight {
		help = layoutTokens(append(base, scrollTokens...), width)
		bodyHeight = budget(help)
	}
	if bodyHeight < 0 {
		// Extremely small frames keep the exit shortcut and drop the summary
		// and the source-range line rather than the body.
		limit := height - len(parts.header) - summaryLines - positionLines - pinned
		if limit < 1 {
			limit = 1
		}
		help = shrinkHelp(help, limit, firstToken(p.exit), width)
		bodyHeight = budget(help)
		if bodyHeight < 0 && summaryLines > 0 {
			summaryLines = 0
			bodyHeight = budget(help)
		}
		if bodyHeight < 0 && positionLines > 0 {
			positionLines = 0
			bodyHeight = budget(help)
		}
		if bodyHeight < 0 {
			bodyHeight = 0
		}
	}

	body, offset, total := m.bodyWindow(p, bodyHeight)
	parts.body = body
	parts.offset = offset
	parts.total = total
	parts.help = help
	parts.tail = append([]string{}, p.tail...)
	if summaryLines > 0 {
		parts.summary = []string{truncateLine(p.summary, width)}
	}
	if positionLines > 0 && len(body) > 0 {
		parts.position = m.previewPosition(p.origin, offset, len(body))
	}
	return parts
}

// contentTotal is the number of body lines before windowing.
func contentTotal(p page) int {
	if p.list != nil {
		total := len(p.list.prefix)
		for _, item := range p.list.items {
			total += len(item)
		}
		return total
	}
	return len(p.body)
}

// bodyWindow returns the body lines that fit in height cells.
func (m model) bodyWindow(p page, height int) ([]string, int, int) {
	if p.list != nil {
		return listWindow(p.list, height)
	}
	if height <= 0 {
		return nil, 0, len(p.body)
	}
	offset := p.offset
	if !p.scrollable {
		offset = 0
	}
	lines, offset := windowLines(p.body, offset, height)
	return lines, offset, len(p.body)
}

// listWindow windows a selectable list so the selected entry stays visible. At
// least one list row always wins over the installed summary.
func listWindow(l *listSection, height int) ([]string, int, int) {
	total := len(l.prefix)
	for _, item := range l.items {
		total += len(item)
	}
	if len(l.items) == 0 {
		lines, _ := windowLines(l.prefix, 0, height)
		return lines, 0, total
	}
	selected := clampInt(l.selected, 0, len(l.items)-1)
	lines := make([]string, 0, height)
	if len(l.prefix) < height {
		lines = append(lines, l.prefix...)
	}
	room := height - len(lines)
	start := selected
	needed := len(l.items[selected])
	for start > 0 && needed+len(l.items[start-1]) <= room {
		start--
		needed += len(l.items[start])
	}
	for i := start; i < len(l.items) && len(lines) < height; i++ {
		lines = append(lines, l.items[i]...)
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	return lines, start, total
}

// previewPosition describes the source line range covered by the visible
// preview screen lines.
func (m model) previewPosition(origin []int, offset, visible int) string {
	start := clampInt(offset, 0, len(origin))
	end := clampInt(offset+visible, start, len(origin))
	if start >= end {
		return ""
	}
	total := origin[len(origin)-1] + 1
	return fmt.Sprintf("lines %d-%d / %d", origin[start]+1, origin[end-1]+1, total)
}

// renderResizeHint replaces every page below the minimum layout size. The exit
// shortcut of the current page is reserved first; the explanation and the title
// use whatever remains, and the title is dropped before the explanation.
func renderResizeHint(p page, width, height int) string {
	if height <= 0 {
		return ""
	}
	help := layoutTokens(p.exit, width)
	if len(help) > height {
		help = help[:height]
	}
	lines := make([]string, 0, height)
	remaining := height - len(help)
	if remaining > 0 {
		message := fmt.Sprintf("Terminal too small (%dx%d). Resize to at least %dx%d.",
			width, height, minimumLayoutWidth, minimumLayoutHeight)
		messageLines := wrapText(message, width)
		if p.title != "" && remaining >= len(messageLines)+1 {
			lines = append(lines, truncateLine(styleTitle.Render(p.title), width))
		}
		room := remaining - len(lines)
		if len(messageLines) > room {
			messageLines = clipLines(messageLines, room, width)
		}
		lines = append(lines, messageLines...)
	}
	lines = append(lines, help...)
	return joinLines(lines, width)
}

// ---------------------------------------------------------------------------
// Pages
// ---------------------------------------------------------------------------

// currentPage selects the page for the current model state.
func (m model) currentPage() page {
	switch {
	case !m.ready:
		return page{
			title:  titleStatus,
			body:   []string{styleDim.Render("Checking mihomo status...")},
			tokens: []string{"q) Quit"},
			exit:   []string{"q) Quit"},
		}
	case m.statusErr != nil:
		return page{
			title:      titleStatus,
			summary:    styleError.Render("✗ Error: " + firstLine(m.statusErr.Error())),
			body:       styleBlock("Error: "+m.statusErr.Error(), m.viewWidth(), styleError),
			scrollable: true,
			offset:     m.readOffset,
			tokens:     []string{"r) Retry", "q) Quit"},
			exit:       []string{"q) Quit"},
		}
	case m.executing != actNone:
		return m.executingPage()
	}
	switch m.mode {
	case modeConfirmUninstall:
		return m.uninstallPage()
	case modeChooseVersion:
		return m.versionChoicePage()
	case modeConfig:
		return m.configPage()
	case modeSchedule:
		return m.schedulePage()
	}
	return m.statusPage()
}

func (m model) View() string {
	view, _ := m.renderPage(m.currentPage())
	return view
}

func (m model) statusView() string {
	view, _ := m.renderPage(m.statusPage())
	return view
}

func (m model) configView() string {
	view, _ := m.renderPage(m.configPage())
	return view
}

func (m model) versionChoiceView() string {
	view, _ := m.renderPage(m.versionChoicePage())
	return view
}

func (m model) scheduleView() string {
	view, _ := m.renderPage(m.schedulePage())
	return view
}

func (m model) uninstallView() string {
	view, _ := m.renderPage(m.uninstallPage())
	return view
}

func (m model) executingView() string {
	view, _ := m.renderPage(m.executingPage())
	return view
}

func (m model) executingPage() page {
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
	width := m.viewWidth()
	frame := spinnerFrames[m.spinnerFrame%len(spinnerFrames)]
	body := []string{styleAccent.Render(label + " " + frame)}
	if m.phaseMsg != "" {
		body = append(body, styleBlock(fmt.Sprintf("[%s] %s", m.phaseLabel, m.phaseMsg), width, styleDim)...)
	}
	tail := []string{}
	if !m.startedAt.IsZero() {
		tail = append(tail, styleDim.Render("elapsed "+time.Since(m.startedAt).Round(time.Second).String()))
	}
	return page{title: titleStatus, body: body, tail: tail}
}

func (m model) uninstallPage() page {
	backup := "no"
	if m.keepBackup {
		backup = "yes"
	}
	inner := m.viewWidth() - 4
	if inner < 1 {
		inner = 1
	}
	body := wrapText("Uninstall mihomo?", inner)
	body = append(body, "")
	body = append(body, wrapText("backup: "+backup+"  (b to toggle)", inner)...)
	dialog := strings.Split(styleDialog.Render(strings.Join(body, "\n")), "\n")
	return page{
		title:  titleUninstall,
		body:   dialog,
		tokens: []string{"y) Confirm", "n) Cancel"},
		exit:   []string{"n) Cancel"},
	}
}

func (m model) versionChoicePage() page {
	width := m.viewWidth()
	p := page{
		title:      titleVersions,
		scrollable: true,
		offset:     m.readOffset,
		tokens:     []string{"↑↓) select", "Enter) confirm", "q) cancel"},
		exit:       []string{"q) cancel"},
	}
	prefix := []string{}
	if m.status != nil && m.status.Version != "" {
		prefix = []string{truncateLine("installed: "+m.status.Version, width)}
	}
	switch {
	case m.versionsErr != nil:
		p.body = append(prefix, "")
		p.body = append(p.body, styleBlock("Error loading versions: "+m.versionsErr.Error(), width, styleError)...)
		p.summary = styleError.Render("✗ Version list unavailable")
	case len(m.versions) == 0:
		p.body = append(prefix, "No releases available.")
	default:
		// The list itself is navigated by the selection keys, so it must not
		// advertise the body scrolling shortcuts.
		p.scrollable = false
		p.list = &listSection{prefix: prefix, items: m.versionItems(), selected: m.selectedIdx}
	}
	return p
}

func (m model) versionItems() [][]string {
	width := m.viewWidth() - 2
	if width < 1 {
		width = 1
	}
	items := make([][]string, 0, len(m.versions))
	for i, version := range m.versions {
		lines := make([]string, 0, 2)
		for j, part := range wrapText(version.Tag, width) {
			prefix := "  "
			selected := i == m.selectedIdx
			if selected && j == 0 {
				prefix = "> "
			}
			if selected {
				lines = append(lines, styleSelected.Render(prefix+part))
				continue
			}
			lines = append(lines, prefix+part)
		}
		items = append(items, lines)
	}
	return items
}

// textCell is one labelled status value and the style applied to it.
type textCell struct {
	text  string
	style lipgloss.Style
}

func (m model) statusPage() page {
	p := page{
		title:      titleStatus,
		scrollable: true,
		offset:     m.readOffset,
		exit:       []string{"q) Quit"},
	}
	if m.status == nil {
		p.body = []string{"No status available."}
		p.tokens = []string{"r) Refresh", "q) Quit"}
		return p
	}
	p.body = m.statusCard()
	p.body = append(p.body, m.statusDetails(m.viewWidth())...)
	p.summary = m.statusSummary()
	p.tokens = statusTokens(m.status)
	return p
}

// statusDetails keeps the complete error text in the scrollable body so it can
// be read instead of being dropped by the hint area.
func (m model) statusDetails(width int) []string {
	details := []string{}
	if m.configStatus.ErrorSummary != "" {
		details = append(details, "")
		details = append(details, styleBlock("Config error: "+m.configStatus.ErrorSummary, width, styleError)...)
	}
	if m.execResult == "failed" && m.actionErr != nil {
		details = append(details, "")
		details = append(details, styleBlock("✗ "+m.actionErr.Error(), width, styleError)...)
	}
	return details
}

// statusSummary is the single line shown in the fixed hint area.
func (m model) statusSummary() string {
	switch {
	case m.execResult == "success":
		return styleSuccess.Render("✓ Operation succeeded")
	case m.execResult == "failed" && m.actionErr != nil:
		return styleError.Render("✗ " + firstLine(m.actionErr.Error()))
	case m.configStatus.ErrorSummary != "":
		return styleError.Render("✗ Config error: " + firstLine(m.configStatus.ErrorSummary))
	}
	return ""
}

func (m model) statusCells() []textCell {
	s := m.status
	state := s.InstanceState.String()
	glyph := "●"
	stateStyle := styleStopped
	switch {
	case !s.Installed:
		state, glyph, stateStyle = "not installed", "○", styleDim
	case s.InstanceState == manager.Running:
		stateStyle = styleRunning
	case s.InstanceState == manager.Upgrading:
		glyph, stateStyle = "◐", styleUpgrading
	}
	cells := []textCell{{text: glyph + " " + state, style: stateStyle}}
	if !s.Installed {
		return cells
	}

	version := s.Version
	if version == "" {
		version = "unknown"
	}
	autostart := "off"
	if s.AutoStartEnabled {
		autostart = "on"
	}
	configState := string(m.configStatus.State)
	configStyle := lipgloss.NewStyle()
	switch {
	case configState == "" || m.configErr != nil:
		configState = "unknown"
		configStyle = styleDim
	case m.configStatus.State == manager.ConfigApplyFailed ||
		m.configStatus.State == manager.ConfigValidationFailed ||
		m.configStatus.ErrorSummary != "":
		configStyle = styleError
	}
	scheduleText := scheduleSummary(m.scheduleErr, m.scheduleActive, m.scheduleInterval)
	scheduleStyle := lipgloss.NewStyle()
	if scheduleText == "legacy" || scheduleText == "unknown" {
		scheduleStyle = styleStopped
	}
	return append(cells,
		textCell{text: "version: " + version},
		textCell{text: "autostart: " + autostart},
		textCell{text: "config: " + configState, style: configStyle},
		textCell{text: "schedule: " + scheduleText, style: scheduleStyle},
	)
}

// statusCard keeps the compact two-column card on the standard layout and
// drops to a borderless single column when space is tight.
func (m model) statusCard() []string {
	width := m.viewWidth()
	cells := m.statusCells()
	if m.layoutMode() == layoutStandard {
		if len(cells) == 5 {
			left := []textCell{cells[0], cells[2], cells[4]}
			right := []textCell{cells[1], cells[3]}
			if lines, ok := renderTwoColumnCard(left, right, width); ok {
				return lines
			}
		}
		if lines, ok := renderCard(cells, width); ok {
			return lines
		}
	}
	return renderColumn(cells, width)
}

func renderCell(cell textCell, width int) []string {
	if width < 1 {
		return nil
	}
	return styleLines(wrapText(cell.text, width), cell.style)
}

func renderColumn(cells []textCell, width int) []string {
	lines := make([]string, 0, len(cells))
	for _, cell := range cells {
		lines = append(lines, renderCell(cell, width)...)
	}
	return lines
}

// renderTwoColumnCard renders a compact bordered card. Values wrap inside their
// column so long versions or paths never break the frame.
func renderTwoColumnCard(left, right []textCell, width int) ([]string, bool) {
	inner := width - 4
	if inner < 16 {
		return nil, false
	}
	const gap = 3
	leftWidth := 0
	for _, cell := range left {
		if w := displayWidth(cell.text); w > leftWidth {
			leftWidth = w
		}
	}
	if limit := inner * 2 / 5; leftWidth > limit {
		leftWidth = limit
	}
	rightWidth := inner - leftWidth - gap
	if rightWidth < 10 {
		return nil, false
	}
	rows := make([]string, 0, len(left))
	for i := range left {
		leftLines := renderCell(left[i], leftWidth)
		var rightLines []string
		if i < len(right) {
			rightLines = renderCell(right[i], rightWidth)
		}
		height := len(leftLines)
		if len(rightLines) > height {
			height = len(rightLines)
		}
		for j := 0; j < height; j++ {
			row := ""
			if j < len(leftLines) {
				row = leftLines[j]
			}
			row += strings.Repeat(" ", max(0, leftWidth-displayWidth(row)))
			if j < len(rightLines) {
				row += strings.Repeat(" ", gap) + rightLines[j]
			}
			rows = append(rows, row)
		}
	}
	return strings.Split(styleCard.Render(strings.Join(rows, "\n")), "\n"), true
}

func renderCard(cells []textCell, width int) ([]string, bool) {
	inner := width - 4
	if inner < 8 {
		return nil, false
	}
	lines := make([]string, 0, len(cells))
	for _, cell := range cells {
		lines = append(lines, renderCell(cell, inner)...)
	}
	return strings.Split(styleCard.Render(strings.Join(lines, "\n")), "\n"), true
}

func renderActionToken(key, label string, enabled bool) string {
	token := key + ") " + label
	if !enabled {
		// Unavailable actions stay readable without relying on colour alone.
		return styleDim.Render(token + " (n/a)")
	}
	return token
}

func statusTokens(s *manager.Status) []string {
	if !isInstalled(s) {
		return []string{renderActionToken("i", "Install", true), "r) Refresh", "q) Quit"}
	}
	autostart := "Autostart on"
	if s.AutoStartEnabled {
		autostart = "Autostart off"
	}
	return []string{
		renderActionToken("1", "Start", isActionAllowed(s, actStart)),
		renderActionToken("2", "Stop", isActionAllowed(s, actStop)),
		renderActionToken("3", "Restart", isActionAllowed(s, actRestart)),
		renderActionToken("4", "Reload", isActionAllowed(s, actReload)),
		renderActionToken("5", "Upgrade", true),
		renderActionToken("c", "Config", true),
		renderActionToken("a", autostart, true),
		renderActionToken("t", "Schedule", true),
		renderActionToken("u", "Uninstall", true),
		"r) Refresh",
		"q) Quit",
	}
}

func (m model) configPage() page {
	p := page{
		title:      titleConfig,
		tabs:       m.configTabsLine(),
		scrollable: true,
		offset:     m.readOffset,
		exit:       []string{"q) Back"},
	}
	p.summary = m.configSummary()
	width := m.viewWidth()
	switch m.configTab {
	case configTabSubscription:
		p.body = wrapText("Subscription source: remote or local", width)
		p.body = append(p.body, styleLines(wrapText("e) Edit source with $EDITOR (URL or local subscription-data)", width), styleDim)...)
		p.body = append(p.body, styleLines(wrapText("CLI: mihomo-manager subscription set <url-or-data>", width), styleDim)...)
		p.tokens = []string{"Tab/← →) switch tab", "e) Edit", "r) Refresh preview", "q) Back"}
	case configTabOverride:
		p.body = wrapText("Override file: "+manager.OverrideFilePath, width)
		p.body = append(p.body, styleLines(wrapText("Edit with: mihomo-manager config override edit", width), styleDim)...)
		p.body = append(p.body, "")
		p.body = append(p.body, wrapText("Rules are embedded in the override file's 'rules:' field.", width)...)
		p.tokens = []string{"Tab/← →) switch tab", "q) Back"}
	case configTabPreview:
		p = m.previewPage(p)
	}
	if m.execResult == "failed" && m.actionErr != nil {
		p.body = append(p.body, "")
		p.body = append(p.body, styleBlock("✗ "+m.actionErr.Error(), width, styleError)...)
	}
	return p
}

func (m model) configSummary() string {
	switch m.execResult {
	case "success":
		return styleSuccess.Render("✓ Configuration updated")
	case "failed":
		if m.actionErr != nil {
			return styleError.Render("✗ " + firstLine(m.actionErr.Error()))
		}
		return styleError.Render("✗ Configuration update failed")
	}
	return ""
}

func (m model) configTabsLine() string {
	tabs := []string{"Subscription", "Override", "Preview"}
	parts := make([]string, 0, len(tabs))
	for i, tab := range tabs {
		if configTab(i) == m.configTab {
			parts = append(parts, styleSelected.Render("["+tab+"]"))
			continue
		}
		parts = append(parts, styleDim.Render(" "+tab+" "))
	}
	return strings.Join(parts, " ")
}

func (m model) previewPage(p page) page {
	width := m.viewWidth()
	switch {
	case m.previewErr != nil:
		p.body = styleBlock("✗ "+m.previewErr.Error(), width, styleError)
		p.summary = styleError.Render("✗ Preview error")
		p.tokens = []string{"r) Retry", "q) Back"}
		p.scrollable = true
		p.offset = m.readOffset
	case m.previewContent == "" && m.previewLoading:
		p.body = []string{styleDim.Render("Loading preview...")}
		p.tokens = []string{"q) Back"}
	case m.previewContent == "":
		p.body = []string{styleDim.Render("(empty)")}
		p.tokens = []string{"r) Refresh", "q) Back"}
	default:
		p.body, p.origin = wrapPreview(m.previewContent, width)
		p.offset = m.previewOffset
		p.preview = true
		p.tokens = []string{"r) Refresh", "q) Back"}
	}
	return p
}

// wrapPreview lays the configuration out on screen lines and records which
// source line every screen line came from, so the footer can keep reporting
// source ranges while scrolling uses screen lines.
func wrapPreview(content string, width int) ([]string, []int) {
	source := strings.Split(content, "\n")
	screen := make([]string, 0, len(source))
	origin := make([]int, 0, len(source))
	for line, text := range source {
		for _, part := range wrapText(text, width) {
			screen = append(screen, part)
			origin = append(origin, line)
		}
	}
	return screen, origin
}

func (m model) previewScreenLines(width int) ([]string, []int) {
	return wrapPreview(m.previewContent, width)
}

// previewOffsetForWidth keeps the source line that was at the top of the
// preview in view when the terminal width changes and the content re-wraps.
func (m model) previewOffsetForWidth(previousWidth int) int {
	_, previousOrigin := m.previewScreenLines(previousWidth)
	if len(previousOrigin) == 0 {
		return 0
	}
	index := clampInt(m.previewOffset, 0, len(previousOrigin)-1)
	source := previousOrigin[index]
	_, origin := m.previewScreenLines(m.viewWidth())
	for i, line := range origin {
		if line >= source {
			return i
		}
	}
	return max(0, len(origin)-1)
}

func (m model) schedulePage() page {
	current := scheduleSummary(m.scheduleErr, m.scheduleActive, m.scheduleInterval)
	items := make([][]string, 0, len(schedulePresets))
	for i, preset := range schedulePresets {
		label := formatDuration(preset)
		if i == m.selectedIdx {
			items = append(items, []string{styleSelected.Render("> " + label)})
			continue
		}
		items = append(items, []string{"  " + label})
	}
	return page{
		title: titleSchedule,
		list: &listSection{
			prefix:   wrapText("current: "+current, m.viewWidth()),
			items:    items,
			selected: m.selectedIdx,
		},
		tokens: []string{"↑↓) select", "Enter) apply", "q) cancel"},
		exit:   []string{"q) cancel"},
	}
}

// ---------------------------------------------------------------------------
// Reading offsets
// ---------------------------------------------------------------------------

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

func (m *model) resetViewOffsets() {
	m.readOffset = 0
	m.previewOffset = 0
}

// currentFrame reports the body window the current page renders with.
func (m model) currentFrame() frameInfo {
	_, info := m.renderPage(m.currentPage())
	return info
}

// clampViewOffset keeps the stored reading offset inside the current window,
// which is what a terminal resize changes. The offset used by the last frame is
// authoritative, so the stored value always matches what is on screen.
func (m *model) clampViewOffset() {
	info := m.currentFrame()
	if !info.valid || info.list {
		return
	}
	if info.preview {
		m.previewOffset = info.offset
		return
	}
	m.readOffset = info.offset
}

// scrollBody moves the reading offset of the page body by one line or one page.
// The preview body (and only the preview body) scrolls with previewOffset, so an
// error page shown inside the Preview tab still scrolls normally.
func (m *model) scrollBody(key string) {
	info := m.currentFrame()
	if !info.valid || info.total <= info.bodyHeight {
		return
	}
	step := 0
	switch key {
	case "up", "k":
		step = -1
	case "down", "j":
		step = 1
	case "pgup":
		step = -max(1, info.bodyHeight)
	case "pgdown":
		step = max(1, info.bodyHeight)
	default:
		return
	}
	if info.preview {
		m.previewOffset += step
	} else {
		m.readOffset += step
	}
	m.clampViewOffset()
}

// updateResizeOnly handles keys while the terminal is below the minimum size:
// business actions are refused and only the page's exit shortcut works.
func (m model) updateResizeOnly(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeStatus:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case modeConfirmUninstall:
		if key := msg.String(); key == "n" || key == "N" || key == "q" {
			m.mode = modeStatus
			m.resetViewOffsets()
		}
	default:
		if key := msg.String(); key == "q" || key == "esc" {
			m.mode = modeStatus
			m.resetViewOffsets()
		}
	}
	return m, nil
}

func startTUI(ctx context.Context, ctrl manager.ServiceControl, lifecycle manager.LifecycleManager, cfg manager.ConfigManager, schedule manager.ScheduleManager) error {
	p := tea.NewProgram(model{ctx: ctx, control: ctrl, lifecycle: lifecycle, config: cfg, schedule: schedule}, tea.WithContext(ctx), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
