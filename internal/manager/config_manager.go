package manager

import "context"

type ConfigManager interface {
	SetSubscriptionSource(ctx context.Context, source string) error
	PreviewConfig(ctx context.Context) (string, error)
	UpdateConfig(ctx context.Context) error
	ValidateConfig(ctx context.Context) error
	AdoptConfig(ctx context.Context, force bool) (AdoptReport, error)
}
