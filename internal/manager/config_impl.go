package manager

import "context"

type configManager struct {
	fs       FileSystem
	source   ReleaseSource
	pipeline *configPipeline
}

func NewConfigManager(fs FileSystem, source ReleaseSource, validate ConfigValidator, onReload func(ctx context.Context) error, options ...ConfigManagerOption) ConfigManager {
	pipelineOptions := ConfigPipelineOptions{
		OnReload:  onReload,
		Validator: validate,
	}
	for _, option := range options {
		option(&pipelineOptions)
	}
	pipe := newConfigPipeline(fs, source, pipelineOptions)
	return &configManager{fs: fs, source: source, pipeline: pipe}
}

func (m *configManager) SetSubscriptionSource(ctx context.Context, source string) error {
	return m.pipeline.SetSubscriptionSource(ctx, source)
}

func (m *configManager) PreviewConfig(ctx context.Context) (string, error) {
	return m.pipeline.Preview(ctx)
}

func (m *configManager) UpdateConfig(ctx context.Context) error {
	return m.pipeline.Apply(ctx)
}

func (m *configManager) AdoptConfig(ctx context.Context, force bool) (AdoptReport, error) {
	return m.pipeline.Adopt(ctx, force)
}

func (m *configManager) ValidateConfig(ctx context.Context) error {
	return m.pipeline.Validate(ctx)
}

func (m *configManager) LastConfigApply(ctx context.Context) (ConfigApplyStatus, error) {
	return m.pipeline.LastConfigApply(ctx)
}
