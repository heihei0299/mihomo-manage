//go:build windows

package manager

import "os"

// openLockFile keeps write access on Windows: creating the lock inside the
// protected data directory needs it, and every Windows run is elevated anyway.
func openLockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o644)
}
