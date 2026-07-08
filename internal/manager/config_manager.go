package manager

import "context"

type ConfigManager interface {
	SetSubscriptionSource(ctx context.Context, source string) error
	SetRoutingRules(ctx context.Context, rules string) error
	PreviewConfig(ctx context.Context) (string, error)
	UpdateConfig(ctx context.Context) error
}
