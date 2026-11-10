package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestChownTreeWalksNestedEntries(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "etc")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configFile := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configFile, []byte("mode: rule\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	link := filepath.Join(root, "mihomo")
	if err := os.Symlink(configFile, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	uid, gid := os.Getuid(), os.Getgid()
	if err := chownTree(root, uid, gid); err != nil {
		t.Fatalf("chownTree = %v, want nil", err)
	}

	for _, path := range []string{root, configDir, configFile, link} {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("lstat %s: %v", path, err)
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			t.Fatalf("lstat %s: unexpected stat type %T", path, info.Sys())
		}
		if int(stat.Uid) != uid || int(stat.Gid) != gid {
			t.Errorf("%s owned by %d:%d, want %d:%d", path, stat.Uid, stat.Gid, uid, gid)
		}
	}
}
