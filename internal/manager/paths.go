package manager

import "path/filepath"

const (
	filePermUserRW    = 0644
	filePermUserRWX   = 0755
	filePermPrivateRW = 0600
	dirPermPrivate    = 0700
)

var (
	installRoot, managerRoot, instanceOperationLockFile = platformStoragePaths()

	binaryPath             = filepath.Join(installRoot, "bin", coreExecutableName())
	configDir              = filepath.Join(installRoot, "etc")
	OverrideFilePath       = filepath.Join(configDir, "override.yaml")
	RoutingRulesPath       = filepath.Join(configDir, "rules.txt")
	configYAML             = filepath.Join(configDir, "config.yaml")
	stateDir               = filepath.Join(managerRoot, "state")
	ServiceLogPath         = filepath.Join(managerRoot, "logs", "mihomo.log")
	subscriptionDataFile   = filepath.Join(stateDir, "subscription-data.txt")
	subscriptionURLFile    = filepath.Join(stateDir, "subscription-url.txt")
	subscriptionSourceFile = filepath.Join(stateDir, "subscription-source.txt")
	// This inode must survive uninstall; config and lifecycle share this lock.
	subscriptionUpdateLockFile = instanceOperationLockFile
	configApplyStatusFile      = filepath.Join(stateDir, "config-apply-status.json")
	configApplyTransactionFile = filepath.Join(stateDir, "config-apply-transaction.json")
)
