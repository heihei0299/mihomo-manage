package manager

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBranchableErrorContracts(t *testing.T) {
	t.Run("subscription source not configured", func(t *testing.T) {
		fs := &fakeFileSystem{
			fileExists: map[string]bool{OverrideFilePath: true},
			written:    map[string][]byte{OverrideFilePath: []byte("mode: rule\n")},
		}
		m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, noopReload)

		err := m.UpdateConfig(context.Background())
		if !errors.Is(err, ErrSubscriptionSourceNotConfigured) {
			t.Fatalf("UpdateConfig error = %v, want ErrSubscriptionSourceNotConfigured", err)
		}
	})

	t.Run("schedule requires installed mihomo", func(t *testing.T) {
		m := NewScheduleManagerWithPlatform(
			&fakeFileSystem{},
			&fakePlatformScheduler{},
			"/usr/local/bin/mihomo-manager",
		)

		err := m.SetSchedule(context.Background(), time.Hour)
		if !errors.Is(err, ErrMihomoNotInstalled) {
			t.Fatalf("SetSchedule error = %v, want ErrMihomoNotInstalled", err)
		}
	})

	t.Run("upgrade requires installed mihomo", func(t *testing.T) {
		m := NewLifecycleManager(
			&fakeFileSystem{},
			&fakeCmdRunner{},
			&fakeReleaseSource{},
			&mockServiceManager{}, noopScheduleManager{})

		err := m.Upgrade(context.Background(), "v1.0.0", noopProgress)
		if !errors.Is(err, ErrMihomoNotInstalled) {
			t.Fatalf("Upgrade error = %v, want ErrMihomoNotInstalled", err)
		}
	})

	t.Run("service state errors are branchable", func(t *testing.T) {
		installed := map[string]bool{binaryPath: true}

		running := NewServiceControl(
			&fakeFileSystem{fileExists: installed},
			&fakeCmdRunner{},
			&mockServiceManager{running: true},
			passConfigValidation,
		)
		if err := running.Start(context.Background()); !errors.Is(err, ErrMihomoAlreadyRunning) {
			t.Fatalf("Start error = %v, want ErrMihomoAlreadyRunning", err)
		}

		stopped := NewServiceControl(
			&fakeFileSystem{fileExists: installed},
			&fakeCmdRunner{},
			&mockServiceManager{running: false},
			passConfigValidation,
		)
		if err := stopped.Stop(context.Background()); !errors.Is(err, ErrMihomoNotRunning) {
			t.Fatalf("Stop error = %v, want ErrMihomoNotRunning", err)
		}
	})

	t.Run("unsupported scheduler is typed", func(t *testing.T) {
		err := (unsupportedPlatformScheduler{os: "plan9"}).Set(
			context.Background(),
			time.Hour,
			"/usr/local/bin/mihomo-manager",
		)
		var unsupported UnsupportedPlatformError
		if !errors.As(err, &unsupported) {
			t.Fatalf("Set error = %v, want UnsupportedPlatformError", err)
		}
		if unsupported.Feature != "scheduler" || unsupported.GOOS != "plan9" {
			t.Fatalf("unsupported error = %+v", unsupported)
		}
	})
}
