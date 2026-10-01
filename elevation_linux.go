package main

import (
	"os"
	"os/exec"
)

func elevateForPlatform(args []string) bool {
	if os.Geteuid() == 0 || !needsElevation(args) {
		return false
	}
	sudoPath, err := exec.LookPath("sudo")
	if err != nil {
		return false
	}
	command := exec.Command(sudoPath, os.Args...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		os.Exit(1)
	}
	return true
}
