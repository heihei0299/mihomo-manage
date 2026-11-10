package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/heihei0299/mihomo-manage/internal/manager"
)

// restoreInvokerOwnership hands the manager-owned trees back to the user who
// started the elevated process. Every command runs as root, so without this
// each run would leave root-owned files behind and the user could no longer
// edit /opt/mihomo without sudo.
func restoreInvokerOwnership() {
	uid, gid, ok := invokerIdentity(os.Geteuid(), os.Getenv("SUDO_UID"), os.Getenv("SUDO_GID"))
	if !ok {
		return
	}
	for _, root := range manager.ManagedRoots() {
		if err := chownTree(root, uid, gid); err != nil {
			fmt.Fprintf(os.Stderr, "warning: restoring ownership of %s: %v\n", root, err)
		}
	}
}

// invokerIdentity resolves the unprivileged user that sudo re-executed the
// manager for. It reports false when there is no such user to restore to.
func invokerIdentity(euid int, sudoUID, sudoGID string) (int, int, bool) {
	if euid != 0 {
		return 0, 0, false
	}
	uid, err := strconv.Atoi(sudoUID)
	if err != nil || uid <= 0 {
		return 0, 0, false
	}
	gid, err := strconv.Atoi(sudoGID)
	if err != nil || gid < 0 {
		return 0, 0, false
	}
	return uid, gid, true
}

// chownTree gives every entry below root to uid:gid without following
// symlinks. A missing root is not an error; unreadable entries do not stop the
// remaining repair.
func chownTree(root string, uid, gid int) error {
	var errs []error
	walkErr := filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			if path == root && errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			errs = append(errs, err)
			return nil
		}
		if chownErr := os.Lchown(path, uid, gid); chownErr != nil {
			errs = append(errs, chownErr)
		}
		return nil
	})
	if walkErr != nil {
		errs = append(errs, walkErr)
	}
	return errors.Join(errs...)
}
