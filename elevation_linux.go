package main

import (
	"os"
	"os/exec"
)

func sudoArgs(executable string, exeErr error, rawArgs []string) []string {
	target := ""
	if len(rawArgs) > 0 {
		target = rawArgs[0]
	}
	if exeErr == nil && executable != "" {
		target = executable
	}
	out := make([]string, 0, max(1, len(rawArgs)))
	out = append(out, target)
	if len(rawArgs) > 1 {
		out = append(out, rawArgs[1:]...)
	}
	return out
}

func elevateForPlatform(args []string) bool {
	if os.Geteuid() == 0 || !needsElevation(args) {
		return false
	}
	sudoPath, err := exec.LookPath("sudo")
	if err != nil {
		return false
	}
	exe, exeErr := os.Executable()
	command := exec.Command(sudoPath, sudoArgs(exe, exeErr, os.Args)...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		os.Exit(1)
	}
	return true
}
