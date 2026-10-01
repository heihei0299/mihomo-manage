//go:build !linux && !windows

package manager

import (
	"os"
	"runtime"
)

func lockOperationFile(*os.File) error {
	return UnsupportedPlatformError{Feature: "instance lock", GOOS: runtime.GOOS}
}
func unlockOperationFile(*os.File) error { return nil }
func operationLockBusy(error) bool       { return false }
