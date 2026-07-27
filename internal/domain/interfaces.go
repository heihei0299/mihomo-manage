package domain

import (
	"context"
	"time"
)

// ServiceControl provides high-level service operations for mihomo.
type ServiceControl interface {
	Status(ctx context.Context) (*Status, error)
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	Reload(ctx context.Context) error
	SetAutoStart(ctx context.Context, enabled bool) error
}

// LifecycleManager handles install, upgrade, and uninstall operations.
type LifecycleManager interface {
	Install(ctx context.Context, version string, autoStart bool, onProgress ProgressCallback) error
	InstallFromLocal(ctx context.Context, localPath string, autoStart bool, onProgress ProgressCallback) error
	Uninstall(ctx context.Context, keepBackup bool, onProgress ProgressCallback) error
	Upgrade(ctx context.Context, version string, onProgress ProgressCallback) error
	ListVersions(ctx context.Context) ([]VersionInfo, error)
}

// ConfigManager manages subscription source, routing rules, and config generation.
type ConfigManager interface {
	SetSubscriptionSource(ctx context.Context, source string) error
	SetRoutingRules(ctx context.Context, rules string) error
	PreviewConfig(ctx context.Context) (string, error)
	UpdateConfig(ctx context.Context) error
}

// ScheduleManager controls periodic config refresh scheduling.
type ScheduleManager interface {
	SetSchedule(ctx context.Context, interval time.Duration) error
	StopSchedule(ctx context.Context) error
	ScheduleStatus(ctx context.Context) (time.Duration, bool, error)
}

// ConfigValidator validates a mihomo config file.
type ConfigValidator interface {
	Validate(ctx context.Context, configPath string) error
}

// ConfigPipeline is the internal pipeline for config generation and application.
type ConfigPipeline interface {
	SetSubscriptionSource(ctx context.Context, source string) error
	SetRoutingRules(ctx context.Context, rules string) error
	Preview(ctx context.Context) (string, error)
	Apply(ctx context.Context) error
}

// ServiceManager provides OS-level service management (start/stop/register etc.).
type ServiceManager interface {
	IsRunning(name string) (bool, error)
	Register(name, serviceFilePath string) error
	Unregister(name string) error
	Start(name string) error
	Stop(name string) error
	Restart(name string) error
	Reload(name string) error
	EnableAutoStart(name, serviceFilePath string) error
	DisableAutoStart(name string) error
	AutoStartEnabled(name string) (bool, error)
}
