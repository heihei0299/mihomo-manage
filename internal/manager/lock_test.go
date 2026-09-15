package manager

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

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
