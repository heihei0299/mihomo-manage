package manager

import (
	"errors"
	"fmt"
)

var (
	// ErrConfigUpdateBusy means another config apply already owns the update lock.
	ErrConfigUpdateBusy = errors.New("configuration update already in progress")

	// ErrSubscriptionSourceNotConfigured means config apply has no selected source.
	ErrSubscriptionSourceNotConfigured = errors.New("subscription source is not configured")

	// ErrMihomoNotInstalled means an operation requires an installed mihomo binary.
	ErrMihomoNotInstalled = errors.New("mihomo is not installed")

	// ErrAdoptNeedsConfirmation means a large adopt diff requires an explicit retry.
	ErrAdoptNeedsConfirmation = errors.New("adopt needs confirmation: large diff, use --force")
)

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
