package manager

import "context"

func NewConfigManager(fs FileSystem, source ReleaseSource, validate ConfigValidator, onReload func(ctx context.Context) error, options ...configManagerOption) ConfigManager {
	pipelineOptions := configPipelineOptions{
		OnReload:  onReload,
		Validator: validate,
	}
	for _, option := range options {
		option(&pipelineOptions)
	}
	return newConfigPipeline(fs, source, pipelineOptions)
}
