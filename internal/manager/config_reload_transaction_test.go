package manager

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type reloadTransactionFaultFS struct {
	*fakeFileSystem
	point      string
	fired      bool
	failStatus ConfigApplyState
}

func (fs *reloadTransactionFaultFS) WriteFile(path string, data []byte, perm uint32) error {
	if path == configApplyStatusFile+".tmp" {
		var status ConfigApplyStatus
		if err := json.Unmarshal(data, &status); err != nil {
			return err
		}
		if status.State == fs.failStatus {
			return errors.New("status storage unavailable")
		}
		if !fs.fired && fs.point == "after-reload" && status.State == ConfigApplied {
			fs.fired = true
			panic("simulated process interruption")
		}
	}
	return fs.fakeFileSystem.WriteFile(path, data, perm)
}

func (fs *reloadTransactionFaultFS) Rename(oldPath, newPath string) error {
	if err := fs.fakeFileSystem.Rename(oldPath, newPath); err != nil {
		return err
	}
	if !fs.fired && fs.point == "disk-committed" && newPath == configApplyTransactionFile {
		var transaction configTransactionState
		if err := json.Unmarshal(fs.written[newPath], &transaction); err != nil {
			return err
		}
		if transaction.State == configTransactionCommitted {
			fs.fired = true
			panic("simulated process interruption")
		}
	}
	return nil
}

func (fs *reloadTransactionFaultFS) RemoveAll(path string) error {
	if !fs.fired && fs.point == "status-applied" && path == configApplyTransactionFile {
		fs.fired = true
		panic("simulated process interruption")
	}
	return fs.fakeFileSystem.RemoveAll(path)
}

func TestInterruptedConfigReloadRetainsAttemptStatus(t *testing.T) {
	for _, point := range []string{"disk-committed", "before-reload", "after-reload", "status-applied"} {
		t.Run(point, func(t *testing.T) {
			base := remoteApplyTestFileSystem()
			fs := &reloadTransactionFaultFS{fakeFileSystem: base, point: point}
			dl := &fakeDownloader{content: "proxies:\n  - name: new\n"}
			linkStorage(base, &dl.fakeReleaseSource)
			reloads := 0
			m := newTestConfigManager(fs, dl, &passValidator{}, func(context.Context) error {
				if point == "before-reload" {
					fs.fired = true
					panic("simulated process interruption")
				}
				reloads++
				return nil
			})
			func() {
				defer func() {
					if recover() == nil {
						t.Fatal("expected process interruption")
					}
				}()
				_ = m.UpdateConfig(context.Background())
			}()
			if _, ok := base.written[configApplyTransactionFile]; !ok {
				t.Fatal("interrupted apply lost its journal")
			}
			want := ConfigPendingReload
			if point == "status-applied" {
				want = ConfigApplied
			}
			fresh := newTestConfigManager(base, dl, &passValidator{}, func(context.Context) error {
				t.Fatal("reading status or preview must not reload")
				return nil
			})
			status, err := fresh.LastConfigApply(context.Background())
			if err != nil || status.State != want || status.ConfigHash != configContentHash(string(base.written[configYAML])) || status.SubscriptionHash != configContentHash(string(base.written[subscriptionDataFile])) {
				t.Fatalf("status = %+v, err = %v, want %s for committed files", status, err, want)
			}
			config, subscription := string(base.written[configYAML]), string(base.written[subscriptionDataFile])
			if _, err := fresh.PreviewConfig(context.Background()); err != nil {
				t.Fatal(err)
			}
			if string(base.written[configYAML]) != config || string(base.written[subscriptionDataFile]) != subscription {
				t.Fatal("recovery rolled back committed files")
			}
			requireConfigApplyState(t, fresh, want)
			if _, ok := base.written[configApplyTransactionFile]; ok {
				t.Fatal("recovery did not clean the completed disk transaction")
			}
			if (point == "after-reload" || point == "status-applied") && reloads != 1 {
				t.Fatalf("reloads = %d, want 1", reloads)
			}
		})
	}
}

func TestConfigReloadStatusWriteFailureKeepsRecoveryJournal(t *testing.T) {
	for _, failedState := range []ConfigApplyState{ConfigPendingReload, ConfigApplied} {
		t.Run(string(failedState), func(t *testing.T) {
			base := localApplyTestFileSystem()
			fs := &reloadTransactionFaultFS{fakeFileSystem: base, failStatus: failedState}
			reloaded := false
			m := newTestConfigManager(fs, &fakeReleaseSource{}, &passValidator{}, func(context.Context) error {
				reloaded = true
				return nil
			})
			if err := m.UpdateConfig(context.Background()); err == nil {
				t.Fatal("expected status write failure")
			}
			if reloaded != (failedState == ConfigApplied) {
				t.Fatalf("reloaded = %v for failure writing %s", reloaded, failedState)
			}
			if _, ok := base.written[configApplyTransactionFile]; !ok {
				t.Fatal("status write failure removed the recovery journal")
			}
			fresh := newTestConfigManager(base, &fakeReleaseSource{}, &passValidator{}, noopReload)
			requireConfigApplyState(t, fresh, ConfigPendingReload)
			if _, err := fresh.PreviewConfig(context.Background()); err != nil {
				t.Fatal(err)
			}
			requireConfigApplyState(t, fresh, ConfigPendingReload)
		})
	}
}

func TestCommittedConfigJournalDoesNotTrustEarlierIdenticalConfig(t *testing.T) {
	base := localApplyTestFileSystem()
	pending := newConfigApplyStatus(ConfigPendingReload, "mode: rule\n", nil, nil)
	prior := pending
	prior.State = ConfigApplied
	prior.AttemptedAt = prior.AttemptedAt.Add(-time.Minute)
	base.written[configYAML] = []byte("mode: rule\n")
	base.written[configApplyStatusFile], _ = json.Marshal(prior)
	base.written[configApplyTransactionFile], _ = json.Marshal(configTransactionState{State: configTransactionCommitted, PendingApply: &pending})
	m := newTestConfigManager(base, &fakeReleaseSource{}, &passValidator{}, noopReload)
	requireConfigApplyState(t, m, ConfigPendingReload)
	if _, err := m.PreviewConfig(context.Background()); err != nil {
		t.Fatal(err)
	}
	requireConfigApplyState(t, m, ConfigPendingReload)
}

func TestLegacyCommittedConfigJournalRecoversPendingStatus(t *testing.T) {
	base := localApplyTestFileSystem()
	base.written[configYAML] = []byte("mode: rule\n")
	base.written[configApplyTransactionFile], _ = json.Marshal(configTransactionState{State: configTransactionCommitted})
	m := newTestConfigManager(base, &fakeReleaseSource{}, &passValidator{}, noopReload)
	requireConfigApplyState(t, m, ConfigPendingReload)
	if _, err := m.PreviewConfig(context.Background()); err != nil {
		t.Fatal(err)
	}
	requireConfigApplyState(t, m, ConfigPendingReload)
}
