package main

import (
	"context"
	"fmt"

	"mihomo-manager/internal/manager"

	tea "github.com/charmbracelet/bubbletea"
)

type viewMode int

const (
	modeStatus viewMode = iota
	modeConfirmUninstall
	modeChooseVersion
	modeConfig
)

type configTab int

const (
	configTabSubscription configTab = iota
	configTabTemplate
	configTabRules
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
)

type model struct {
	mgr            manager.Manager
	status         *manager.Status
	statusErr      error
	ready          bool
	executing      action
	execResult     string
	actionErr      error
	phaseLabel     string
	phaseMsg       string

	mode           viewMode
	keepBackup     bool
	versions       []manager.VersionInfo
	selectedIdx    int
	configTab      configTab
	previewContent string
}

type statusMsg struct {
	status *manager.Status
	err    error
}

type actionDoneMsg struct {
	action action
	err    error
}

type progressMsg struct {
	phase   manager.InstallationPhase
	message string
	err     error
}

type versionsMsg struct {
	versions []manager.VersionInfo
	err      error
}

type configPreviewMsg struct {
	content string
	err     error
}

func (m model) Init() tea.Cmd {
	return fetchStatusCmd(m.mgr)
}

func fetchStatusCmd(mgr manager.Manager) tea.Cmd {
	return func() tea.Msg {
		s, err := mgr.Status(context.Background())
		return statusMsg{status: s, err: err}
	}
}

func fetchConfigPreview(mgr manager.Manager) tea.Cmd {
	return func() tea.Msg {
		s, err := mgr.PreviewConfig(context.Background())
		return configPreviewMsg{content: s, err: err}
	}
}

func fetchVersionsCmd(mgr manager.Manager) tea.Cmd {
	return func() tea.Msg {
		v, err := mgr.ListVersions(context.Background())
		return versionsMsg{versions: v, err: err}
	}
}

func execActionCmd(mgr manager.Manager, a action, progressCh chan<- progressMsg, version string) tea.Cmd {
	return func() tea.Msg {
		var err error
		ctx := context.Background()
		switch a {
		case actStart:
			err = mgr.Start(ctx)
		case actStop:
			err = mgr.Stop(ctx)
		case actRestart:
			err = mgr.Restart(ctx)
		case actReload:
			err = mgr.Reload(ctx)
		case actInstall:
			err = mgr.Install(ctx, version, func(e manager.ProgressEvent) {
				if progressCh != nil {
					progressCh <- progressMsg{phase: e.Phase, message: e.Message, err: e.Error}
				}
			})
		case actUpgrade:
			err = mgr.Upgrade(ctx, version, func(e manager.ProgressEvent) {
				if progressCh != nil {
					progressCh <- progressMsg{phase: e.Phase, message: e.Message, err: e.Error}
				}
			})
		case actUninstall:
			err = mgr.Uninstall(ctx, false, func(e manager.ProgressEvent) {
				if progressCh != nil {
					progressCh <- progressMsg{phase: e.Phase, message: e.Message, err: e.Error}
				}
			})
		}
		if progressCh != nil {
			close(progressCh)
		}
		return actionDoneMsg{action: a, err: err}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "c":
			if isInstalled(m.status) {
				m.mode = modeConfig
				m.configTab = configTabSubscription
				m.previewContent = ""
				return m, fetchConfigPreview(m.mgr)
			}
		case "r":
			return m, fetchStatusCmd(m.mgr)
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
				m.selectedIdx = 0
				return m, fetchVersionsCmd(m.mgr)
			}
		case "i":
			if !isInstalled(m.status) {
				return m.startAction(actInstall, "latest")
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
		m.execResult = ""
		m.actionErr = nil
		return m, nil

	case versionsMsg:
		if msg.err == nil {
			m.versions = msg.versions
		}
		return m, nil

	case configPreviewMsg:
		if msg.err == nil {
			m.previewContent = msg.content
		}
		return m, nil

	case progressMsg:
		m.phaseLabel = msg.phase.String()
		m.phaseMsg = msg.message
		return m, nil

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
		return m, fetchStatusCmd(m.mgr)
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

func (m model) updateConfigMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.mode = modeStatus
		return m, nil
	case "tab", "right":
		m.configTab = (m.configTab + 1) % 4
		return m, nil
	case "left":
		m.configTab = (m.configTab - 1 + 4) % 4
		return m, nil
	case "r":
		return m, fetchConfigPreview(m.mgr)
	}
	return m, nil
}

func (m model) startAction(a action, version string) (tea.Model, tea.Cmd) {
	m.executing = a
	m.mode = modeStatus
	ch := make(chan progressMsg, 20)
	return m, tea.Batch(
		progressReaderCmd(ch),
		execActionCmd(m.mgr, a, ch, version),
	)
}

func progressReaderCmd(ch <-chan progressMsg) tea.Cmd {
	return func() tea.Msg {
		e, ok := <-ch
		if !ok {
			return nil
		}
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
	switch a {
	case actStart:
		return s.InstanceState == manager.Stopped
	case actStop:
		return s.InstanceState == manager.Running
	case actRestart:
		return s.InstanceState == manager.Running
	case actReload:
		return s.InstanceState == manager.Running
	}
	return false
}

func (m model) View() string {
	if !m.ready {
		return "Checking mihomo status...\n\nPress q to quit"
	}
	if m.statusErr != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit", m.statusErr)
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
	}
	phase := ""
	if m.phaseMsg != "" {
		phase = fmt.Sprintf("\n[%s] %s", m.phaseLabel, m.phaseMsg)
	}
	return fmt.Sprintf("Status: %s%s\n\nPress r to refresh, q to quit", label, phase)
}

func (m model) uninstallView() string {
	backup := "no"
	if m.keepBackup {
		backup = "yes"
	}
	return fmt.Sprintf(
		"Uninstall mihomo?\n\n"+
			"Keep backup: %s  (b to toggle)\n\n"+
			"y) Confirm    n) Cancel",
		backup,
	)
}

func (m model) versionChoiceView() string {
	s := "Select version (enter to confirm, q to cancel):\n\n"
	for i, v := range m.versions {
		prefix := "  "
		if i == m.selectedIdx {
			prefix = "> "
		}
		s += fmt.Sprintf("%s%s\n", prefix, v.Tag)
	}
	return s
}

func (m model) statusView() string {
	s := m.status
	stateStr := s.InstanceState.String()
	version := s.Version
	if version == "" {
		version = "unknown"
	}

	var actions string
	if !s.Installed {
		actions = "\ni) Install"
	} else {
		if isActionAllowed(s, actStart) {
			actions += "\n1) Start"
		}
		if isActionAllowed(s, actStop) {
			actions += "\n2) Stop"
		}
		if isActionAllowed(s, actRestart) {
			actions += "\n3) Restart"
		}
		if isActionAllowed(s, actReload) {
			actions += "\n4) Reload"
		}
		actions += "\n5) Upgrade"
		actions += "\nu) Uninstall"
	}

	result := ""
	if m.execResult == "success" {
		result = "\n✓ Operation succeeded"
	} else if m.execResult == "failed" {
		result = fmt.Sprintf("\n✗ %v", m.actionErr)
	}

	return fmt.Sprintf(
		"┌────────────────────────┐\n"+
			"│ mihomo: %-12s │\n"+
			"│ version: %-12s │\n"+
			"└────────────────────────┘%s%s\n\n"+
			"r) Refresh    q) Quit",
		stateStr, version, actions, result,
	)
}

func (m model) configView() string {
	tabs := []string{"Subscription", "Template", "Rules", "Preview"}
	tabLine := ""
	for i, t := range tabs {
		sep := "  "
		if i == int(m.configTab) {
			tabLine += fmt.Sprintf("[%s]", t)
		} else {
			tabLine += fmt.Sprintf(" %s ", t)
		}
		if i < len(tabs)-1 {
			tabLine += sep
		}
	}
	tabLine += "\n\n"

	var content string
	switch m.configTab {
	case configTabSubscription:
		content = "Subscription: /opt/mihomo-manager/state/subscription-data.txt\n"
		content += "Edit with: mihomo-manager subscription set <url-or-data>\n"
	case configTabTemplate:
		content = "Config template: /opt/mihomo/etc/config-template.yaml\n"
		content += "Edit with: mihomo-manager template edit\n"
	case configTabRules:
		content = "Routing rules: /opt/mihomo/etc/rules.txt\n"
		content += "Edit with: mihomo-manager rules edit\n"
	case configTabPreview:
		if m.previewContent == "" {
			content = "Loading preview...\n"
		} else if len(m.previewContent) > 1000 {
			content = m.previewContent[:1000] + "\n... (truncated)"
		} else {
			content = m.previewContent
		}
	}

	return tabLine + content + "\n\nTab/← → switch tab  r) refresh preview  q) back"
}

func startTUI(mgr manager.Manager) error {
	p := tea.NewProgram(model{mgr: mgr})
	_, err := p.Run()
	return err
}
