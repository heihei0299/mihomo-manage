package service

import (
	"strings"
	"testing"

	"github.com/anomalyco/mihomo-manager/internal/domain"
)

func TestReleaseURLIncludesOSArch(t *testing.T) {
	url := releaseURL("linux", "amd64", "v1.19.27")

	if !strings.Contains(url, "linux") {
		t.Error("BUG 5: release URL should contain OS (linux)")
	}
	if !strings.Contains(url, "amd64") {
		t.Error("BUG 5: release URL should contain arch (amd64)")
	}
	if !strings.Contains(url, ".gz") {
		t.Error("BUG 5: release URL should have .gz extension")
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		output   string
		expected string
	}{
		{"Mihomo Meta v1.18.0 linux amd64", "v1.18.0"},
		{"mihomo v1.18.0", "v1.18.0"},
		{"Mihomo Meta V1.18.0 linux amd64 go1.22.0", "V1.18.0"},
		{"unknown output format", "unknown output format"},
		{"", ""},
	}

	for _, tt := range tests {
		cmd := &fakeCmdRunner{cmdOutput: tt.output}
		got, err := domain.ParseVersion(cmd, "/dummy")
		if err != nil {
			t.Errorf("ParseVersion(%q) unexpected error: %v", tt.output, err)
			continue
		}
		if got != tt.expected {
			t.Errorf("ParseVersion(%q) = %q, want %q", tt.output, got, tt.expected)
		}
	}
}

func TestParseVersionError(t *testing.T) {
	cmd := &fakeCmdRunner{cmdErr: testError{"command failed"}}
	_, err := domain.ParseVersion(cmd, "/dummy")
	if err == nil {
		t.Error("expected error, got nil")
	}
}
