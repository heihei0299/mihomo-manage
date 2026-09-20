package manager

import "context"

func NewConfigManager(fs FileSystem, source ReleaseSource, validate ConfigValidator, onReload func(ctx context.Context) error, options ...configManagerOption) ConfigManager {
	if validate == nil {
		panic("manager: config validator is required")
	}
	if onReload == nil {
		panic("manager: config reload function is required")
	}
	pipelineOptions := configPipelineOptions{
		OnReload:  onReload,
		Validator: validate,
	}
	for _, option := range options {
		option(&pipelineOptions)
	}
	return newConfigPipeline(fs, source, pipelineOptions)
}
