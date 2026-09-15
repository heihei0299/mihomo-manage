package manager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const configUpdateLockWait = 2 * time.Second

type fileConfigUpdateLock struct {
	path string
}

func NewFileConfigUpdateLock() ConfigUpdateLock {
	return &fileConfigUpdateLock{path: subscriptionUpdateLockFile}
}

func (l *fileConfigUpdateLock) Acquire(ctx context.Context) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(l.path), 0755); err != nil {
		return nil, fmt.Errorf("creating update lock directory: %w", err)
	}
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening update lock: %w", err)
	}

	tryLock := func() error {
		return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	}
	if err := tryLock(); err == nil {
		return func() {
			_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
			_ = file.Close()
		}, nil
	} else if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
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
			return nil, ErrConfigUpdateBusy
		case <-ticker.C:
			if err := tryLock(); err == nil {
				return func() {
					_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
					_ = file.Close()
				}, nil
			} else if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
				_ = file.Close()
				return nil, fmt.Errorf("acquiring update lock: %w", err)
			}
		}
	}
}
