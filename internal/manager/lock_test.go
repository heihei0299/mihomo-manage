package manager

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestFileConfigUpdateLockHonorsCancellationWhileWaiting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config-update.lock")
	first := &fileConfigUpdateLock{path: path}
	second := &fileConfigUpdateLock{path: path}

	release, err := first.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	defer release()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, err := second.Acquire(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second Acquire error = %v, want deadline exceeded", err)
	}
}

func TestFileConfigUpdateLockReleasesAndHonorsCancellation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config-update.lock")
	first := &fileConfigUpdateLock{path: path}
	second := &fileConfigUpdateLock{path: path}

	release, err := first.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := second.Acquire(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("second Acquire error = %v, want cancellation", err)
	}

	release()
	if release, err := second.Acquire(context.Background()); err != nil {
		t.Fatalf("second Acquire after release failed: %v", err)
	} else {
		release()
	}
}
