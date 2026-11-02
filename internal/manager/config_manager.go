package manager

import (
	"context"
	"time"
)

// ConfigApplyState records the externally meaningful result of an apply.
//
// State contract:
//
//	validation-failed -> staged config was rejected; current config is unchanged
//	apply-failed      -> apply failed before a successful commit/reload
//	pending-reload    -> config was committed, but runtime reload is unconfirmed
//	applied           -> config was committed and reload succeeded
//
// An applied status may still carry ErrorSummary when only post-commit cleanup
// failed. In that case the runtime state is applied and the error is a warning
// about cleanup, not a failed transaction.
type ConfigApplyState string

const (
	ConfigApplied          ConfigApplyState = "applied"
	ConfigPendingReload    ConfigApplyState = "pending-reload"
	ConfigValidationFailed ConfigApplyState = "validation-failed"
	ConfigApplyFailed      ConfigApplyState = "apply-failed"
	ConfigUnknown          ConfigApplyState = "unknown"
)

type ConfigApplyStatus struct {
	State            ConfigApplyState `json:"state"`
	AttemptedAt      time.Time        `json:"attempted_at"`
	ConfigHash       string           `json:"config_hash"`
	SubscriptionHash string           `json:"subscription_hash,omitempty"`
	ErrorSummary     string           `json:"error_summary,omitempty"`
	// SubscriptionStale reports that the stored subscription data no longer
	// matches the data the last apply rendered. It is derived at read time and
	// never persisted.
	SubscriptionStale bool `json:"-"`
}

type ConfigManager interface {
	SetSubscriptionSource(ctx context.Context, source string) error
	PreviewConfig(ctx context.Context) (string, error)
	// UpdateConfig refreshes the subscription and applies it.
	UpdateConfig(ctx context.Context) error
	// FetchSubscription refreshes subscription data without touching config. It
	// reports whether data was fetched; a local source has nothing to fetch.
	FetchSubscription(ctx context.Context) (bool, error)
	// ApplyConfig applies stored subscription data without downloading.
	ApplyConfig(ctx context.Context) error
	ValidateConfig(ctx context.Context) error
	LastConfigApply(ctx context.Context) (ConfigApplyStatus, error)
	AdoptConfig(ctx context.Context, force bool) (AdoptReport, error)
}
