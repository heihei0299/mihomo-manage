//go:build !windows

package manager

import "context"

func newWindowsServiceStrategy() osStrategy { return nil }

func checkWindowsServiceTarget(context.Context, string) error {
	return UnsupportedPlatformError{Feature: "service", GOOS: "windows"}
}

// RunWindowsService is reached only by the Windows service entry point.
func RunWindowsService() error {
	return UnsupportedPlatformError{Feature: "service", GOOS: "windows"}
}
