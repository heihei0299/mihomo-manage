package main

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func elevateForPlatform(args []string) bool {
	if !needsElevation(args) || windows.GetCurrentProcessToken().IsElevated() {
		return false
	}
	fmt.Fprintln(os.Stderr, "this command requires administrator privileges; run it from an Administrator PowerShell or Command Prompt")
	os.Exit(1)
	return true
}
