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

// prepareManagedOwnership arranges for every file the manager creates in its
// user data trees to belong to the person who invoked it. An elevated run also
// repairs what earlier releases left root-owned, so one sudo command is enough
// to hand an existing installation back.
func prepareManagedOwnership() {
	if os.Geteuid() != 0 {
		warnUnownedUserRoots()
		return
	}
	uid, gid, ok := resolveOwnerIdentity(os.Geteuid(), os.Getenv("SUDO_UID"), os.Getenv("SUDO_GID"), manager.LoadOwnerRecord)
	if !ok {
		return
	}
	manager.SetManagedOwner(uid, gid)
	if err := manager.SaveOwnerRecord(uid, gid); err != nil {
		fmt.Fprintf(os.Stderr, "warning: recording managed owner: %v\n", err)
	}
	if err := manager.EnsureInstanceOperationLock(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: preparing instance lock: %v\n", err)
	}
	for _, root := range manager.UserDataRoots() {
		if err := chownTree(root, uid, gid); err != nil {
			fmt.Fprintf(os.Stderr, "warning: restoring ownership of %s: %v\n", root, err)
		}
	}
	if err := manager.PruneStaleConfigStaging(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: pruning stale config staging: %v\n", err)
	}
}

// resolveOwnerIdentity finds the user that should own the managed data. An
// explicit sudo invoker wins; otherwise the user recorded by an earlier run
// covers root-initiated work such as the scheduled subscription update.
func resolveOwnerIdentity(euid int, sudoUID, sudoGID string, recorded func() (int, int, bool)) (int, int, bool) {
	if euid != 0 {
		return 0, 0, false
	}
	if uid, gid, ok := invokerIdentity(euid, sudoUID, sudoGID); ok {
		return uid, gid, true
	}
	if recorded != nil {
		if uid, gid, ok := recorded(); ok {
			return uid, gid, true
		}
	}
	return 0, 0, false
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

// chownTree gives every entry below root to uid:gid without following symlinks.
// A missing root is not an error; unreadable entries do not stop the remaining
// repair.
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
