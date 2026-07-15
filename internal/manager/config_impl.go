package manager

import "context"

type configManager struct {
	fs       FileSystem
	gh       GitHubReleases
	pipeline *configPipeline
}

func NewConfigManager(fs FileSystem, gh GitHubReleases, validate ConfigValidator, onReload func(ctx context.Context) error) ConfigManager {
	pipe := newConfigPipeline(fs, gh, ConfigPipelineOptions{
		OnReload:  onReload,
		Validator: validate,
	})
	return &configManager{fs: fs, gh: gh, pipeline: pipe}
}

func (m *configManager) SetSubscriptionSource(ctx context.Context, source string) error {
	return m.pipeline.SetSubscriptionSource(ctx, source)
}

func (m *configManager) SetRoutingRules(ctx context.Context, rules string) error {
	return m.pipeline.SetRoutingRules(ctx, rules)
}

func (m *configManager) PreviewConfig(ctx context.Context) (string, error) {
	return m.pipeline.Preview(ctx)
}

func (m *configManager) UpdateConfig(ctx context.Context) error {
	return m.pipeline.Apply(ctx)
}
