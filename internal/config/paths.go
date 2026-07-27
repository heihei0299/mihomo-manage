package config

// File permission constants.
const (
	FilePermUserRW  = 0644
	FilePermUserRWX = 0755
)

// Path constants for mihomo-manager.
const (
	BinaryPath            = "/opt/mihomo/bin/mihomo"
	ConfigDir             = "/opt/mihomo/etc"
	ConfigTemplatePath    = "/opt/mihomo/etc/config-template.yaml"
	ConfigYAML            = "/opt/mihomo/etc/config.yaml"
	DefaultServiceUnitPath = "/etc/systemd/system/mihomo.service"
	ServiceName           = "mihomo"
	StateDir              = "/opt/mihomo-manager/state"
	SubscriptionDataFile  = "/opt/mihomo-manager/state/subscription-data.txt"
	SubscriptionURLFile   = "/opt/mihomo-manager/state/subscription-url.txt"
	RoutingRulesPath      = "/opt/mihomo/etc/rules.txt"
	ScheduleFile          = "/opt/mihomo-manager/state/schedule.txt"
)

// DefaultReleaseTemplate is the default URL template for downloading mihomo releases.
var DefaultReleaseTemplate = "https://github.com/MetaCubeX/mihomo/releases/download/{version}/mihomo-{os}-{arch}-{version}.gz"
