package manager

import (
	"context"
	"errors"
	"testing"
)

func TestSetSubscriptionSourceUsesConfigUpdateLock(t *testing.T) {
	lock := &fakeConfigUpdateLock{}
	fs := &fakeFileSystem{}
	m := NewConfigManager(fs, &fakeReleaseSource{}, nil, nil, WithConfigUpdateLock(lock))

	if err := m.SetSubscriptionSource(context.Background(), "local subscription"); err != nil {
		t.Fatalf("SetSubscriptionSource failed: %v", err)
	}
	if !lock.acquired || !lock.released {
		t.Fatalf("lock state = acquired:%v released:%v, want both true", lock.acquired, lock.released)
	}
}

func TestSetSubscriptionSourceBusyLeavesStateUntouched(t *testing.T) {
	lock := &fakeConfigUpdateLock{err: ErrConfigUpdateBusy}
	fs := &fakeFileSystem{written: map[string][]byte{subscriptionDataFile: []byte("old")}}
	m := NewConfigManager(fs, &fakeReleaseSource{}, nil, nil, WithConfigUpdateLock(lock))

	if err := m.SetSubscriptionSource(context.Background(), "new subscription"); !errors.Is(err, ErrConfigUpdateBusy) {
		t.Fatalf("SetSubscriptionSource error = %v, want busy", err)
	}
	if string(fs.written[subscriptionDataFile]) != "old" || len(fs.written) != 1 {
		t.Fatalf("state after busy set = %v, want unchanged", fs.written)
	}
}

func TestAdoptConfigUsesConfigUpdateLock(t *testing.T) {
	lock := &fakeConfigUpdateLock{}
	fs := &fakeFileSystem{
		fileExists: map[string]bool{
			configYAML:             true,
			OverrideFilePath:       true,
			subscriptionSourceFile: true,
			subscriptionDataFile:   true,
		},
		written: map[string][]byte{
			configYAML:             []byte("mode: direct\n"),
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
		},
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, nil, nil, WithConfigUpdateLock(lock))

	if _, err := m.AdoptConfig(context.Background(), false); err != nil {
		t.Fatalf("AdoptConfig failed: %v", err)
	}
	if !lock.acquired || !lock.released {
		t.Fatalf("lock state = acquired:%v released:%v, want both true", lock.acquired, lock.released)
	}
}

func TestConfigWriteOperationsPropagateLockCancellation(t *testing.T) {
	for _, test := range []struct {
		name string
		run  func(manager ConfigManager) error
	}{
		{name: "set subscription", run: func(m ConfigManager) error {
			return m.SetSubscriptionSource(context.Background(), "local")
		}},
		{name: "adopt config", run: func(m ConfigManager) error {
			_, err := m.AdoptConfig(context.Background(), false)
			return err
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := NewConfigManager(&fakeFileSystem{}, &fakeReleaseSource{}, nil, nil, WithConfigUpdateLock(&fakeConfigUpdateLock{err: context.Canceled}))
			if err := test.run(m); !errors.Is(err, context.Canceled) {
				t.Fatalf("operation error = %v, want cancellation", err)
			}
		})
	}
}

func TestAdoptConfigBusyLeavesOverrideUntouched(t *testing.T) {
	lock := &fakeConfigUpdateLock{err: ErrConfigUpdateBusy}
	fs := &fakeFileSystem{
		fileExists: map[string]bool{configYAML: true, OverrideFilePath: true, subscriptionSourceFile: true, subscriptionDataFile: true},
		written: map[string][]byte{
			configYAML:             []byte("mode: direct\n"),
			OverrideFilePath:       []byte("mode: rule\n"),
			subscriptionSourceFile: []byte("local\n"),
			subscriptionDataFile:   []byte("mode: rule\n"),
		},
	}
	m := NewConfigManager(fs, &fakeReleaseSource{}, nil, nil, WithConfigUpdateLock(lock))

	if _, err := m.AdoptConfig(context.Background(), false); !errors.Is(err, ErrConfigUpdateBusy) {
		t.Fatalf("AdoptConfig error = %v, want busy", err)
	}
	if string(fs.written[OverrideFilePath]) != "mode: rule\n" {
		t.Fatalf("override after busy adopt = %q, want unchanged", fs.written[OverrideFilePath])
	}
}
