//go:build !windows

package manager

import "os"

// openLockFile opens the shared lock read-only. Flock needs a descriptor, not
// write access, so a root-owned lock stays usable by an unprivileged run.
func openLockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0o644)
}
