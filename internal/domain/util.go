package domain

import (
	"fmt"
	"strings"
	"time"
)

// cmdRunner is the minimal interface needed for ParseVersion.
type cmdRunner interface {
	RunCommand(name string, args ...string) (string, error)
}

// LooksLikeURL returns true if s has an http:// or https:// prefix.
func LooksLikeURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// LooksLikeVersion returns true if s starts with v/V followed by a digit.
func LooksLikeVersion(s string) bool {
	if len(s) < 2 {
		return false
	}
	if s[0] != 'v' && s[0] != 'V' {
		return false
	}
	return s[1] >= '0' && s[1] <= '9'
}

// ParseVersion runs the binary with -v and extracts a version string.
func ParseVersion(cmd cmdRunner, binaryPath string) (string, error) {
	out, err := cmd.RunCommand(binaryPath, "-v")
	if err != nil {
		return "", err
	}
	parts := strings.Fields(out)
	for _, p := range parts {
		if LooksLikeVersion(p) {
			return p, nil
		}
	}
	return out, nil
}

// Timestamp returns the current Unix timestamp as a string.
func Timestamp() string {
	return fmt.Sprintf("%d", time.Now().Unix())
}
