//go:build !linux && !windows

package main

func elevateForPlatform([]string) bool { return false }
