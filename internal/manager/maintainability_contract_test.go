package manager

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBranchableErrorContracts(t *testing.T) {
	t.Run("subscription source not configured", func(t *testing.T) {
		fs := &fakeFileSystem{
			fileExists: map[string]bool{OverrideFilePath: true},
			written:    map[string][]byte{OverrideFilePath: []byte("mode: rule\n")},
		}
		m := NewConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, nil)

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
			&mockServiceManager{},
		)

		err := m.Upgrade(context.Background(), "v1.0.0", noopProgress)
		if !errors.Is(err, ErrMihomoNotInstalled) {
			t.Fatalf("Upgrade error = %v, want ErrMihomoNotInstalled", err)
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

func TestInstallRollbackContract(t *testing.T) {
	primary := errors.New("primary failure")
	stopFailure := errors.New("stop rollback failed")
	removeFailure := errors.New("remove rollback failed")

	tests := []struct {
		name        string
		fs          *fakeFileSystem
		svc         *mockServiceManager
		wantWrapped error
		wantText    string
	}{
		{
			name: "clean rollback preserves primary",
			fs:   &fakeFileSystem{},
			svc:  &mockServiceManager{},
		},
		{
			name:        "service rollback failure is preserved",
			fs:          &fakeFileSystem{},
			svc:         &mockServiceManager{stopErr: stopFailure},
			wantWrapped: stopFailure,
			wantText:    "rollback failed",
		},
		{
			name:        "filesystem rollback failure is preserved",
			fs:          &fakeFileSystem{removeErr: removeFailure},
			svc:         &mockServiceManager{},
			wantWrapped: removeFailure,
			wantText:    "rollback failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &lifecycleManager{fs: tt.fs, svcMgr: tt.svc}
			err := m.rollbackInstall(context.Background(), "test phase", primary)

			if !errors.Is(err, primary) {
				t.Fatalf("rollback error = %v, want primary failure preserved", err)
			}
			if tt.wantWrapped != nil && !errors.Is(err, tt.wantWrapped) {
				t.Fatalf("rollback error = %v, want %v preserved", err, tt.wantWrapped)
			}
			if tt.wantText != "" && !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("rollback error = %v, want %q", err, tt.wantText)
			}
		})
	}
}
