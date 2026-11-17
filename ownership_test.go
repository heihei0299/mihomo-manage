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

func TestResolveOwnerIdentity(t *testing.T) {
	record := func(uid, gid int, ok bool) func() (int, int, bool) {
		return func() (int, int, bool) { return uid, gid, ok }
	}
	cases := []struct {
		name     string
		euid     int
		sudoUID  string
		sudoGID  string
		recorded func() (int, int, bool)
		wantUID  int
		wantGID  int
		wantOK   bool
	}{
		{"unprivileged process does no handover", 1000, "1000", "1000", record(2000, 2000, true), 0, 0, false},
		{"sudo invoker wins", 0, "1000", "1000", record(2000, 2001, true), 1000, 1000, true},
		{"record covers root-initiated runs", 0, "", "", record(2000, 2001, true), 2000, 2001, true},
		{"root without a record keeps root", 0, "", "", record(0, 0, false), 0, 0, false},
		{"malformed sudo falls back to the record", 0, "shial", "", record(2000, 2001, true), 2000, 2001, true},
		{"nil record is safe", 0, "", "", nil, 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uid, gid, ok := resolveOwnerIdentity(tc.euid, tc.sudoUID, tc.sudoGID, tc.recorded)
			if ok != tc.wantOK || uid != tc.wantUID || gid != tc.wantGID {
				t.Fatalf("resolveOwnerIdentity(%d, %q, %q) = (%d, %d, %v), want (%d, %d, %v)",
					tc.euid, tc.sudoUID, tc.sudoGID, uid, gid, ok, tc.wantUID, tc.wantGID, tc.wantOK)
			}
		})
	}
}
