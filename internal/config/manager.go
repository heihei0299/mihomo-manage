package config

import (
	"context"

	"github.com/anomalyco/mihomo-manager/internal/domain"
	"github.com/anomalyco/mihomo-manager/internal/infra"
)

// manager implements domain.ConfigManager.
type manager struct {
	fs       infra.FileSystem
	gh       infra.ReleaseRepo
	pipeline domain.ConfigPipeline
}

// NewManager returns a domain.ConfigManager that orchestrates subscription, routing,
// and config generation via the pipeline.
func NewManager(fs infra.FileSystem, gh infra.ReleaseRepo, validate domain.ConfigValidator, onReload func(ctx context.Context) error) domain.ConfigManager {
	pipe := NewPipeline(fs, gh, ConfigPipelineOptions{
		OnReload:  onReload,
		Validator: validate,
	})
	return &manager{fs: fs, gh: gh, pipeline: pipe}
}

func (m *manager) SetSubscriptionSource(ctx context.Context, source string) error {
	return m.pipeline.SetSubscriptionSource(ctx, source)
}

func (m *manager) SetRoutingRules(ctx context.Context, rules string) error {
	return m.pipeline.SetRoutingRules(ctx, rules)
}

func (m *manager) PreviewConfig(ctx context.Context) (string, error) {
	return m.pipeline.Preview(ctx)
}

func (m *manager) UpdateConfig(ctx context.Context) error {
	return m.pipeline.Apply(ctx)
}
