package manager

import (
	"context"
	"time"
)

type ConfigApplyState string

const (
	ConfigApplied          ConfigApplyState = "applied"
	ConfigPendingReload    ConfigApplyState = "pending-reload"
	ConfigValidationFailed ConfigApplyState = "validation-failed"
	ConfigUnknown          ConfigApplyState = "unknown"
)

type ConfigApplyStatus struct {
	State        ConfigApplyState `json:"state"`
	AttemptedAt  time.Time        `json:"attempted_at"`
	ConfigHash   string           `json:"config_hash"`
	ErrorSummary string           `json:"error_summary,omitempty"`
}

type ConfigManager interface {
	SetSubscriptionSource(ctx context.Context, source string) error
	PreviewConfig(ctx context.Context) (string, error)
	UpdateConfig(ctx context.Context) error
	ValidateConfig(ctx context.Context) error
	LastConfigApply(ctx context.Context) (ConfigApplyStatus, error)
	AdoptConfig(ctx context.Context, force bool) (AdoptReport, error)
}
