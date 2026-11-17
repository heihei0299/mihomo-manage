package manager

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

const configUpdateLockWait = 2 * time.Second

type fileOperationLock struct {
	path string
	busy error
}

func NewFileConfigUpdateLock() ConfigUpdateLock {
	return &fileOperationLock{path: subscriptionUpdateLockFile, busy: ErrConfigUpdateBusy}
}

func NewFileInstanceOperationLock() OperationLock {
	return &fileOperationLock{path: instanceOperationLockFile, busy: ErrInstanceBusy}
}

func (l *fileOperationLock) Acquire(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	file, err := openOperationLock(l.path)
	if err != nil {
		return nil, fmt.Errorf("opening operation lock: %w", err)
	}

	tryLock := func() error {
		return lockOperationFile(file)
	}
	if err := tryLock(); err == nil {
		return func() {
			_ = unlockOperationFile(file)
			_ = file.Close()
		}, nil
	} else if !operationLockBusy(err) {
		_ = file.Close()
		return nil, fmt.Errorf("acquiring update lock: %w", err)
	}

	deadline := time.NewTimer(configUpdateLockWait)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, ctx.Err()
		case <-deadline.C:
			_ = file.Close()
			if l.busy != nil {
				return nil, l.busy
			}
			return nil, ErrConfigUpdateBusy
		case <-ticker.C:
			if err := tryLock(); err == nil {
				return func() {
					_ = unlockOperationFile(file)
					_ = file.Close()
				}, nil
			} else if !operationLockBusy(err) {
				_ = file.Close()
				return nil, fmt.Errorf("acquiring update lock: %w", err)
			}
		}
	}
}

// EnsureInstanceOperationLock creates the platform lock with the permissions
// unprivileged runs need to open it. Linux keeps the lock under /run, which only
// an elevated process can create after a reboot.
func EnsureInstanceOperationLock() error {
	file, err := createOperationLock(instanceOperationLockFile)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	// Legacy releases created the lock directory 0700, which keeps every
	// unprivileged run out even after the file itself became readable.
	return os.Chmod(filepath.Dir(instanceOperationLockFile), 0o755)
}

// openOperationLock opens the shared lock file. Flock only needs a descriptor,
// not write access, so the file stays readable for every user even when an
// elevated run owns it.
func openOperationLock(path string) (*os.File, error) {
	file, err := createOperationLock(path)
	if err == nil {
		return file, nil
	}
	if !errors.Is(err, fs.ErrPermission) {
		return nil, err
	}
	// A fresh boot leaves /run root-only. Serializing through the manager state
	// tree still keeps unprivileged writers apart until an elevated run
	// recreates the platform lock.
	return createOperationLock(fallbackLockPath())
}

func createOperationLock(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := openLockFile(path)
	if err != nil {
		return nil, err
	}
	if err := file.Chmod(0o644); err != nil && !errors.Is(err, fs.ErrPermission) {
		_ = file.Close()
		return nil, err
	}
	if err := managedOwner.apply(path); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func fallbackLockPath() string {
	return filepath.Join(stateDir, "instance.lock")
}
