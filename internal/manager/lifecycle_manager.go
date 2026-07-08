package manager

import "context"

type LifecycleManager interface {
	Install(ctx context.Context, version string, autoStart bool, onProgress ProgressCallback) error
	Uninstall(ctx context.Context, keepBackup bool, onProgress ProgressCallback) error
	Upgrade(ctx context.Context, version string, onProgress ProgressCallback) error
	ListVersions(ctx context.Context) ([]VersionInfo, error)
}
