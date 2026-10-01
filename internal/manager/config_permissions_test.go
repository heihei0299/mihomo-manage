//go:build linux

package manager

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type rootedPermissionFS struct {
	OSSystem
	root string
}

func (f rootedPermissionFS) path(p string) string {
	return filepath.Join(f.root, strings.TrimPrefix(p, "/"))
}
func (f rootedPermissionFS) FileExists(p string) (bool, error) {
	return f.OSSystem.FileExists(f.path(p))
}
func (f rootedPermissionFS) ReadFile(p string) ([]byte, error) { return f.OSSystem.ReadFile(f.path(p)) }
func (f rootedPermissionFS) WriteFile(p string, d []byte, m uint32) error {
	return f.OSSystem.WriteFile(f.path(p), d, m)
}
func (f rootedPermissionFS) RemoveAll(p string) error { return f.OSSystem.RemoveAll(f.path(p)) }
func (f rootedPermissionFS) Rename(a, b string) error { return f.OSSystem.Rename(f.path(a), f.path(b)) }
func (f rootedPermissionFS) MkdirAll(p string, m uint32) error {
	return f.OSSystem.MkdirAll(f.path(p), m)
}
func (f rootedPermissionFS) Chmod(p string, m uint32) error { return f.OSSystem.Chmod(f.path(p), m) }

func TestConfigMigratesLegacyCredentialPermissions(t *testing.T) {
	fs := rootedPermissionFS{root: t.TempDir()}
	for _, dir := range []string{stateDir, configDir} {
		if err := os.MkdirAll(fs.path(dir), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(fs.path(dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for path, data := range map[string]string{subscriptionURLFile: "https://example.invalid/sub?token=synthetic", subscriptionSourceFile: "remote\n", subscriptionDataFile: "port: 7890\n", configYAML: "port: 7890\n", OverrideFilePath: "mode: rule\n", configYAML + ".bak.legacy": "synthetic-backup"} {
		if err := os.WriteFile(fs.path(path), []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)
	if _, err := cfg.PreviewConfig(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{subscriptionURLFile, subscriptionSourceFile, subscriptionDataFile, configYAML, OverrideFilePath} {
		info, err := os.Stat(fs.path(path))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("%s mode=%o", path, info.Mode().Perm())
		}
	}
	for _, path := range []string{stateDir, configDir} {
		info, err := os.Stat(fs.path(path))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0700 {
			t.Fatalf("%s mode=%o", path, info.Mode().Perm())
		}
	}
}

func TestSubscriptionCreatesPrivateCredentialFile(t *testing.T) {
	fs := rootedPermissionFS{root: t.TempDir()}
	cfg := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)
	if err := cfg.SetSubscriptionSource(context.Background(), "https://example.invalid/?token=synthetic"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(fs.path(subscriptionURLFile))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}

func TestOSSystemWriteFileTightensExistingPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private")
	if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := (OSSystem{}).WriteFile(path, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "new" {
		t.Fatalf("data=%q err=%v", data, err)
	}
}
