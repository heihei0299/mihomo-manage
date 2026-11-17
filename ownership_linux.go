//go:build linux

package main

import (
	"fmt"
	"os"
	"syscall"

	"github.com/heihei0299/mihomo-manage/internal/manager"
)

// warnUnownedUserRoots points an unprivileged run at the one-time elevated
// command that hands legacy root-owned entries back to the invoking user.
func warnUnownedUserRoots() {
	self, err := os.Executable()
	if err != nil {
		self = "mihomo-manager"
	}
	for _, root := range manager.UserOwnedPaths() {
		info, err := os.Lstat(root)
		if err != nil {
			continue
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || int(stat.Uid) == os.Geteuid() {
			continue
		}
		fmt.Fprintf(os.Stderr, "warning: %s belongs to uid %d; run 'sudo %s status' once to hand the managed tree back\n", root, stat.Uid, self)
	}
}
