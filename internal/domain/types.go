package domain

// InstanceState represents the current state of a mihomo instance.
type InstanceState int

const (
	Stopped   InstanceState = iota
	Running
	Upgrading
)

func (s InstanceState) String() string {
	switch s {
	case Stopped:
		return "stopped"
	case Running:
		return "running"
	case Upgrading:
		return "upgrading"
	default:
		return "unknown"
	}
}

// InstallationPhase represents a step in the install/uninstall/upgrade lifecycle.
type InstallationPhase int

const (
	PhaseFetch InstallationPhase = iota
	PhaseDeploy
	PhaseBootstrap
	PhaseRegister
	PhaseEnableAutoStart
	PhaseStart
	PhaseUpgradeCheck
	PhaseUpgradeFetch
	PhaseUpgradeStop
	PhaseUpgradeReplace
	PhaseUpgradeStart
	PhaseUninstallStop
	PhaseUninstallDeregister
	PhaseUninstallCleanup
)

func (p InstallationPhase) String() string {
	switch p {
	case PhaseFetch:
		return "fetch"
	case PhaseDeploy:
		return "deploy"
	case PhaseBootstrap:
		return "bootstrap"
	case PhaseRegister:
		return "register"
	case PhaseEnableAutoStart:
		return "enable-auto-start"
	case PhaseStart:
		return "start"
	case PhaseUpgradeCheck:
		return "check"
	case PhaseUpgradeFetch:
		return "fetch"
	case PhaseUpgradeStop:
		return "stop"
	case PhaseUpgradeReplace:
		return "replace"
	case PhaseUpgradeStart:
		return "start"
	case PhaseUninstallStop:
		return "stop"
	case PhaseUninstallDeregister:
		return "deregister"
	case PhaseUninstallCleanup:
		return "cleanup"
	default:
		return "unknown"
	}
}

// ProgressEvent describes one progress step during install/upgrade/uninstall.
type ProgressEvent struct {
	Phase   InstallationPhase
	Message string
	Error   error
}

// ProgressCallback is called with each ProgressEvent during a lifecycle operation.
type ProgressCallback func(ProgressEvent)

// VersionInfo describes a single mihomo release version.
type VersionInfo struct {
	Tag string
}

// Status describes the current state of a mihomo installation.
type Status struct {
	InstanceState     InstanceState
	Installed         bool
	Version           string
	AutoStartEnabled  bool
}
