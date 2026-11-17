//go:build !linux

package main

// warnUnownedUserRoots is Linux-only: other platforms use ACLs instead of
// numeric ownership.
func warnUnownedUserRoots() {}
