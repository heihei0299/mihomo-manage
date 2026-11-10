package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInvokerIdentity(t *testing.T) {
	cases := []struct {
		name    string
		euid    int
		sudoUID string
		sudoGID string
		wantOK  bool
		wantUID int
		wantGID int
	}{
		{"unprivileged process", 1000, "1000", "1000", false, 0, 0},
		{"root via sudo", 0, "1000", "1000", true, 1000, 1000},
		{"root without sudo", 0, "", "", false, 0, 0},
		{"root invoking root", 0, "0", "0", false, 0, 0},
		{"missing gid", 0, "1000", "", false, 0, 0},
		{"malformed uid", 0, "shial", "1000", false, 0, 0},
		{"negative gid", 0, "1000", "-1", false, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uid, gid, ok := invokerIdentity(tc.euid, tc.sudoUID, tc.sudoGID)
			if ok != tc.wantOK || uid != tc.wantUID || gid != tc.wantGID {
				t.Fatalf("invokerIdentity(%d, %q, %q) = (%d, %d, %v), want (%d, %d, %v)",
					tc.euid, tc.sudoUID, tc.sudoGID, uid, gid, ok, tc.wantUID, tc.wantGID, tc.wantOK)
			}
		})
	}
}

func TestChownTreeMissingRootIsNotAnError(t *testing.T) {
	root := filepath.Join(t.TempDir(), "not-installed")
	if err := chownTree(root, os.Getuid(), os.Getgid()); err != nil {
		t.Fatalf("chownTree(missing root) = %v, want nil", err)
	}
}
