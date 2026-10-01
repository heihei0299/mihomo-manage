//go:build !windows

package manager

func protectPathPermissions(string, uint32) error { return nil }
