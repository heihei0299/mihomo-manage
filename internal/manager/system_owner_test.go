package manager

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"
)

func TestParseOwnerRecord(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantUID int
		wantGID int
		wantOK  bool
	}{
		{"record written by an elevated run", "1000:1000\n", 1000, 1000, true},
		{"padded fields", " 1000 : 1001 \n", 1000, 1001, true},
		{"empty record", "", 0, 0, false},
		{"missing gid", "1000", 0, 0, false},
		{"root is not a managed owner", "0:0\n", 0, 0, false},
		{"negative gid", "1000:-1\n", 0, 0, false},
		{"non numeric", "shial:shial\n", 0, 0, false},
		{"extra field", "1000:1000:1000\n", 0, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			uid, gid, ok := parseOwnerRecord(tc.raw)
			if ok != tc.wantOK || uid != tc.wantUID || gid != tc.wantGID {
				t.Fatalf("parseOwnerRecord(%q) = (%d, %d, %v), want (%d, %d, %v)",
					tc.raw, uid, gid, ok, tc.wantUID, tc.wantGID, tc.wantOK)
			}
		})
	}
}

func TestUserDataRootsKeepBinariesRootOwned(t *testing.T) {
	roots := UserDataRoots()
	for _, want := range []string{configDir, stateDir} {
		if !slices.Contains(roots, want) {
			t.Errorf("UserDataRoots() = %v, want it to contain %s", roots, want)
		}
	}
	managed := ownerIdentity{roots: roots}
	for _, path := range []string{binaryPath, backupDir, filepath.Join(backupDir, "mihomo.bak")} {
		if managed.within(path) {
			t.Errorf("%s must stay root-owned, but it is inside the user data trees", path)
		}
	}
}

func TestOSSystemHandsCreatedEntriesToManagedOwner(t *testing.T) {
	root := t.TempDir()
	managedOwner = ownerIdentity{uid: 4242, gid: 4242, roots: []string{root}, active: true}
	var chowned []string
	original := chownPath
	chownPath = func(path string, uid, gid int) error {
		if uid != 4242 || gid != 4242 {
			t.Fatalf("chownPath(%s) got %d:%d, want 4242:4242", path, uid, gid)
		}
		chowned = append(chowned, path)
		return nil
	}
	t.Cleanup(func() {
		chownPath = original
		ClearManagedOwner()
	})

	dir := filepath.Join(root, "deeper")
	file := filepath.Join(dir, "config.yaml")
	if err := (OSSystem{}).MkdirAll(dir, dirPermPrivate); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := (OSSystem{}).WriteFile(file, []byte("mode: rule\n"), filePermPrivateRW); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	for _, want := range []string{dir, file} {
		if !slices.Contains(chowned, want) {
			t.Errorf("chownPath was not called for %s; calls: %v", want, chowned)
		}
	}
}

func TestOSSystemLeavesSystemPathsToRoot(t *testing.T) {
	managedRoot := t.TempDir()
	outside := t.TempDir()
	managedOwner = ownerIdentity{uid: 4242, gid: 4242, roots: []string{managedRoot}, active: true}
	calls := 0
	original := chownPath
	chownPath = func(string, int, int) error {
		calls++
		return nil
	}
	t.Cleanup(func() {
		chownPath = original
		ClearManagedOwner()
	})

	if err := (OSSystem{}).WriteFile(filepath.Join(outside, "mihomo.service"), []byte("[Unit]\n"), filePermUserRW); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := (OSSystem{}).MkdirAll(filepath.Join(outside, "system"), filePermUserRWX); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if calls != 0 {
		t.Errorf("chownPath ran %d times for paths outside the user data trees", calls)
	}
}

func TestOSSystemKeepsOwnershipWhenNoOwnerIsConfigured(t *testing.T) {
	ClearManagedOwner()
	calls := 0
	original := chownPath
	chownPath = func(string, int, int) error {
		calls++
		return nil
	}
	t.Cleanup(func() { chownPath = original })

	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := (OSSystem{}).WriteFile(file, []byte("mode: rule\n"), filePermPrivateRW); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if calls != 0 {
		t.Errorf("chownPath ran %d times without a configured owner", calls)
	}
}

func TestOperationLockFileIsReadableByUnprivilegedRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "instance.lock")
	file, err := openOperationLock(path)
	if err != nil {
		t.Fatalf("openOperationLock: %v", err)
	}
	defer file.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat lock: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o644 {
		t.Errorf("lock mode = %o, want 644 so flock works without root", mode)
	}
	reader, err := os.Open(path)
	if err != nil {
		t.Fatalf("reopening lock read-only: %v", err)
	}
	_ = reader.Close()
}

func TestOpenOperationLockFallsBackWhenPlatformLockIsUnwritable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write into a 0500 directory")
	}
	stateRoot := t.TempDir()
	original := stateDir
	stateDir = stateRoot
	t.Cleanup(func() { stateDir = original })

	blocked := filepath.Join(t.TempDir(), "run")
	if err := os.Mkdir(blocked, 0o500); err != nil {
		t.Fatalf("mkdir blocked: %v", err)
	}
	file, err := openOperationLock(filepath.Join(blocked, "instance.lock"))
	if err != nil {
		t.Fatalf("openOperationLock = %v, want the state lock fallback", err)
	}
	defer file.Close()
	want := filepath.Join(stateRoot, "instance.lock")
	if file.Name() != want {
		t.Fatalf("fallback lock = %s, want %s", file.Name(), want)
	}
}

func TestPruneStaleConfigStagingKeepsJournalReferenceAndFreshEntries(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, configStagingPrefix+"stale")
	fresh := filepath.Join(dir, configStagingPrefix+"fresh")
	referenced := filepath.Join(dir, configStagingPrefix+"referenced")
	for _, path := range []string{stale, fresh, referenced} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
	}
	past := time.Now().Add(-2 * staleStagingAge)
	for _, path := range []string{stale, referenced} {
		if err := os.Chtimes(path, past, past); err != nil {
			t.Fatalf("chtimes %s: %v", path, err)
		}
	}
	journal := filepath.Join(dir, "config-apply-transaction.json")
	content := "{\"state\":\"prepared\",\"staged_config_dir\":" + strconv.Quote(referenced) + "}"
	if err := os.WriteFile(journal, []byte(content), 0o600); err != nil {
		t.Fatalf("write journal: %v", err)
	}

	if err := pruneStaleConfigStaging(dir, journal, time.Now()); err != nil {
		t.Fatalf("pruneStaleConfigStaging: %v", err)
	}
	if _, err := os.Stat(stale); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stale staging directory survived: %v", err)
	}
	for _, path := range []string{fresh, referenced} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s was pruned: %v", path, err)
		}
	}
}

func TestPruneStaleConfigStagingKeepsEntriesWhenJournalIsUnreadable(t *testing.T) {
	dir := t.TempDir()
	stale := filepath.Join(dir, configStagingPrefix+"stale")
	if err := os.MkdirAll(stale, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	past := time.Now().Add(-2 * staleStagingAge)
	if err := os.Chtimes(stale, past, past); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	journal := filepath.Join(dir, "config-apply-transaction.json")
	if err := os.WriteFile(journal, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("write journal: %v", err)
	}

	if err := pruneStaleConfigStaging(dir, journal, time.Now()); err != nil {
		t.Fatalf("pruneStaleConfigStaging: %v", err)
	}
	if _, err := os.Stat(stale); err != nil {
		t.Errorf("staging directory was pruned while the journal was unreadable: %v", err)
	}
}

func TestPruneStaleConfigStagingIgnoresMissingConfigDir(t *testing.T) {
	dir := t.TempDir()
	err := pruneStaleConfigStaging(filepath.Join(dir, "absent"), filepath.Join(dir, "journal"), time.Now())
	if err != nil {
		t.Fatalf("pruneStaleConfigStaging = %v, want nil for a missing config directory", err)
	}
}
