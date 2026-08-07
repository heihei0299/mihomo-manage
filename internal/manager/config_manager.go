package manager

import "context"

type ConfigManager interface {
	SetSubscriptionSource(ctx context.Context, source string) error
	PreviewConfig(ctx context.Context) (string, error)
	UpdateConfig(ctx context.Context) error
}
