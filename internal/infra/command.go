package infra

import (
	"os/exec"
	"strings"
)

// CommandRunner provides command execution.
type CommandRunner interface {
	RunCommand(name string, args ...string) (string, error)
	RunCommandIgnoreExit(name string, args ...string) (string, error)
}

// OSCommandRunner implements CommandRunner using the real OS.
type OSCommandRunner struct{}

// NewCommandRunner returns a new OSCommandRunner.
func NewCommandRunner() *OSCommandRunner { return &OSCommandRunner{} }

func (OSCommandRunner) RunCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (OSCommandRunner) RunCommandIgnoreExit(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return string(out), nil
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
