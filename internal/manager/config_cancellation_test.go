package manager

import (
	"context"
	"errors"
	"testing"
)

type blockingSubscriptionDownloader struct {
	fakeReleaseSource
	started chan struct{}
}

func (d *blockingSubscriptionDownloader) Download(ctx context.Context, url, dest string) error {
	close(d.started)
	<-ctx.Done()
	return ctx.Err()
}

type blockingConfigValidator struct {
	started chan struct{}
}

func (v *blockingConfigValidator) Validate(ctx context.Context, path string) error {
	close(v.started)
	<-ctx.Done()
	return ctx.Err()
}

type fakeConfigUpdateLock struct {
	err             error
	acquired        bool
	acquiredContext context.Context
	released        bool
}

func (l *fakeConfigUpdateLock) Acquire(ctx context.Context) (func(), error) {
	l.acquired = true
	l.acquiredContext = ctx
	if l.err != nil {
		return nil, l.err
	}
	return func() { l.released = true }, nil
}

func TestUpdateConfigDownloadHonorsCancellation(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true, subscriptionSourceFile: true, subscriptionURLFile: true},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("remote\n"),
			subscriptionURLFile:    []byte("https://example.com/sub.yaml"),
		},
	}
	dl := &blockingSubscriptionDownloader{started: make(chan struct{})}
	m := NewConfigManager(fs, dl, &passValidator{}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- m.UpdateConfig(ctx) }()
	<-dl.started
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("UpdateConfig error = %v, want cancellation", err)
	}
	requireConfigApplyFailure(t, m)
}

func TestUpdateConfigValidationHonorsCancellation(t *testing.T) {
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true, subscriptionSourceFile: true, subscriptionDataFile: true},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
		},
	}
	validator := &blockingConfigValidator{started: make(chan struct{})}
	m := NewConfigManager(fs, &fakeReleaseSource{}, validator, nil)
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { result <- m.UpdateConfig(ctx) }()
	<-validator.started
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatalf("UpdateConfig error = %v, want cancellation", err)
	}
	requireConfigApplyFailure(t, m)
}

func TestUpdateConfigPassesContextToLock(t *testing.T) {
	lock := &fakeConfigUpdateLock{}
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
		},
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil, WithConfigUpdateLock(lock))
	type contextKey struct{}
	key := contextKey{}
	ctx := context.WithValue(context.Background(), key, "marker")

	if err := m.UpdateConfig(ctx); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}
	if lock.acquiredContext == nil || lock.acquiredContext.Value(key) != "marker" {
		t.Fatalf("lock context = %v, want caller context", lock.acquiredContext)
	}
}

func TestUpdateConfigReturnsBusyWhenLockUnavailable(t *testing.T) {
	lock := &fakeConfigUpdateLock{err: ErrConfigUpdateBusy}
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
		},
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil, WithConfigUpdateLock(lock))

	if err := m.UpdateConfig(context.Background()); !errors.Is(err, ErrConfigUpdateBusy) {
		t.Fatalf("UpdateConfig error = %v, want busy error", err)
	}
	if !lock.acquired {
		t.Fatal("UpdateConfig should attempt to acquire the update lock")
	}
	if _, exists := fs.written[configYAML]; exists {
		t.Fatal("busy update should not write generated config")
	}
}

func TestUpdateConfigReleasesLockAfterSuccess(t *testing.T) {
	lock := &fakeConfigUpdateLock{}
	fs := &fakeFileSystem{
		fileExists: map[string]bool{OverrideFilePath: true},
		written: map[string][]byte{
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
		},
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil, WithConfigUpdateLock(lock))

	if err := m.UpdateConfig(context.Background()); err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}
	if !lock.released {
		t.Fatal("UpdateConfig should release the update lock")
	}
}
