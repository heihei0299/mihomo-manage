package manager

import (
	"context"
	"fmt"
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
	if err := (OSSystem{}).MkdirAll(filepath.Dir(l.path), dirPermPrivate); err != nil {
		return nil, fmt.Errorf("creating update lock directory: %w", err)
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, filePermPrivateRW)
	if err != nil {
		return nil, fmt.Errorf("opening update lock: %w", err)
	}
	if err := (OSSystem{}).Chmod(l.path, filePermPrivateRW); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("protecting update lock: %w", err)
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
