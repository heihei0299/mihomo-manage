package manager

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ownerIdentity is the user that receives ownership of everything the manager
// creates inside UserDataRoots. An elevated process writes as root, so without
// this handover every run would leave files the invoking user cannot edit.
type ownerIdentity struct {
	uid    int
	gid    int
	roots  []string
	active bool
}

var managedOwner ownerIdentity

// chownPath is a seam so tests can assert handovers without dropping files to
// another user.
var chownPath = os.Lchown

// SetManagedOwner makes the OS seam hand entries created inside the user data
// trees to uid:gid. It belongs to the elevated process; an unprivileged one
// already creates files as its own user.
func SetManagedOwner(uid, gid int) {
	managedOwner = ownerIdentity{uid: uid, gid: gid, roots: UserDataRoots(), active: true}
}

// ClearManagedOwner disables the handover.
func ClearManagedOwner() {
	managedOwner = ownerIdentity{}
}

func (o ownerIdentity) apply(path string) error {
	if !o.active || !o.within(path) {
		return nil
	}
	if err := chownPath(path, o.uid, o.gid); err != nil {
		return fmt.Errorf("handing %s to %d:%d: %w", path, o.uid, o.gid, err)
	}
	return nil
}

func (o ownerIdentity) within(path string) bool {
	clean := filepath.Clean(path)
	for _, root := range o.roots {
		cleanRoot := filepath.Clean(root)
		if clean == cleanRoot || strings.HasPrefix(clean, cleanRoot+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

// OwnerRecordPath is where the manager remembers which user owns the user data
// trees, so runs that start as plain root (the systemd update timer, cron) hand
// new files to that user instead of leaving them root-owned.
func OwnerRecordPath() string {
	return filepath.Join(stateDir, "owner")
}

// LoadOwnerRecord reports the user recorded by an earlier elevated run.
func LoadOwnerRecord() (uid, gid int, ok bool) {
	data, err := os.ReadFile(OwnerRecordPath())
	if err != nil {
		return 0, 0, false
	}
	return parseOwnerRecord(string(data))
}

func parseOwnerRecord(raw string) (int, int, bool) {
	fields := strings.Split(strings.TrimSpace(raw), ":")
	if len(fields) != 2 {
		return 0, 0, false
	}
	uid, err := strconv.Atoi(strings.TrimSpace(fields[0]))
	if err != nil || uid <= 0 {
		return 0, 0, false
	}
	gid, err := strconv.Atoi(strings.TrimSpace(fields[1]))
	if err != nil || gid < 0 {
		return 0, 0, false
	}
	return uid, gid, true
}

// SaveOwnerRecord persists the managed owner. The record itself carries the
// recorded owner, so an unprivileged manager can read it back.
func SaveOwnerRecord(uid, gid int) error {
	if err := (OSSystem{}).MkdirAll(stateDir, dirPermPrivate); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}
	record := fmt.Sprintf("%d:%d\n", uid, gid)
	if err := (OSSystem{}).WriteFile(OwnerRecordPath(), []byte(record), filePermPrivateRW); err != nil {
		return fmt.Errorf("writing owner record: %w", err)
	}
	return nil
}

const (
	configStagingPrefix = ".mihomo-config-staging-"
	// A staging directory lives for seconds. Anything this old was abandoned by
	// an interrupted apply, so removing it cannot disturb a running one.
	staleStagingAge = 24 * time.Hour
)

// PruneStaleConfigStaging removes staging directories abandoned by interrupted
// applies. The directory a pending transaction still refers to is kept, and
// nothing is pruned when that journal cannot be understood.
func PruneStaleConfigStaging() error {
	return pruneStaleConfigStaging(configDir, configApplyTransactionFile, time.Now())
}

func pruneStaleConfigStaging(dir, transactionPath string, now time.Time) error {
	referenced, ok := referencedStagingDir(transactionPath)
	if !ok {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var errs []error
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), configStagingPrefix) {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if path == referenced {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if now.Sub(info.ModTime()) < staleStagingAge {
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// referencedStagingDir reports the staging directory of the pending config
// transaction. ok is false when the journal exists but cannot be understood, so
// the caller skips pruning instead of deleting a directory still in use.
func referencedStagingDir(transactionPath string) (string, bool) {
	data, err := os.ReadFile(transactionPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", true
		}
		return "", false
	}
	var transaction configTransactionState
	if err := json.Unmarshal(data, &transaction); err != nil {
		return "", false
	}
	return transaction.StagedConfigDir, true
}
