package manager

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrConfigUpdateBusy means another config apply already owns the update lock.
	ErrConfigUpdateBusy = errors.New("configuration update already in progress")

	// ErrSubscriptionSourceNotConfigured means config apply has no selected source.
	ErrSubscriptionSourceNotConfigured = errors.New("subscription source is not configured")

	// ErrMihomoNotInstalled means an operation requires an installed mihomo binary.
	ErrMihomoNotInstalled = errors.New("mihomo is not installed")

	// ErrMihomoAlreadyInstalled means installation would replace an existing
	// managed installation or its configuration.
	ErrMihomoAlreadyInstalled = errors.New("mihomo is already installed")

	// ErrMihomoAlreadyRunning means a start operation found the instance running.
	ErrMihomoAlreadyRunning = errors.New("mihomo is already running")

	// ErrMihomoNotRunning means an operation requires a running mihomo instance.
	ErrMihomoNotRunning = errors.New("mihomo is not running")

	// ErrAdoptNeedsConfirmation means a large adopt diff requires an explicit retry.
	ErrAdoptNeedsConfirmation = errors.New("adopt needs confirmation: large diff, use --force")
)

// LegacyScheduleError reports a schedule that still uses the legacy scheduler.
type LegacyScheduleError struct {
	Interval time.Duration
}

func (e LegacyScheduleError) Error() string {
	return fmt.Sprintf("legacy schedule configured for every %v; run subscription schedule --interval %v to activate it", e.Interval, e.Interval)
}

// UnsupportedPlatformError is returned when an operation has no implementation
// for the current operating system. Callers can use errors.As when they need to
// distinguish unsupported-platform behavior from an operational failure.
type UnsupportedPlatformError struct {
	Feature string
	GOOS    string
}

func (e UnsupportedPlatformError) Error() string {
	return fmt.Sprintf("unsupported %s platform: %s", e.Feature, e.GOOS)
}
