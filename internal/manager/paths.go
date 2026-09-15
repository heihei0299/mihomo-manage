package manager

const (
	filePermUserRW  = 0644
	filePermUserRWX = 0755

	binaryPath                 = "/opt/mihomo/bin/mihomo"
	configDir                  = "/opt/mihomo/etc"
	OverrideFilePath           = "/opt/mihomo/etc/override.yaml"
	RoutingRulesPath           = "/opt/mihomo/etc/rules.txt"
	configYAML                 = "/opt/mihomo/etc/config.yaml"
	stateDir                   = "/opt/mihomo-manager/state"
	subscriptionDataFile       = "/opt/mihomo-manager/state/subscription-data.txt"
	subscriptionURLFile        = "/opt/mihomo-manager/state/subscription-url.txt"
	subscriptionSourceFile     = "/opt/mihomo-manager/state/subscription-source.txt"
	subscriptionUpdateLockFile = "/opt/mihomo-manager/state/config-update.lock"
	configApplyStatusFile      = "/opt/mihomo-manager/state/config-apply-status.json"
)
