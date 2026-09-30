package manager

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestInterruptedUpgradeRecoversBeforeInstallChecks(t *testing.T) {
	for _, running := range []bool{false, true} {
		for _, point := range []string{"Stopped", "Replacing binary", "Binary replaced", "Starting mihomo"} {
			if !running && point == "Starting mihomo" {
				continue
			}
			t.Run(point+map[bool]string{true: "/running", false: "/stopped"}[running], func(t *testing.T) {
				fs := &fakeFileSystem{fileExists: map[string]bool{binaryPath: true, configDir: true}, written: map[string][]byte{binaryPath: []byte("binary-v1"), configYAML: []byte("mode: rule\n")}}
				source := &fakeReleaseSource{downloadData: fakeReleaseArchiveWith("binary-v2")}
				linkStorage(fs, source)
				svc := &mockServiceManager{running: running}
				m := newTestLifecycleManager(fs, &fakeCmdRunner{}, source, svc, noopScheduleManager{})
				crashed := false
				func() {
					defer func() {
						if recover() != nil {
							crashed = true
						}
					}()
					_ = m.Upgrade(context.Background(), "v2", func(e ProgressEvent) {
						if e.Message == point {
							panic("process interrupted")
						}
					})
				}()
				if !crashed {
					t.Fatal("crash point not reached")
				}
				if _, ok := fs.written[lifecycleUpgradeTransactionFile]; !ok {
					t.Fatal("missing recovery journal")
				}
				fresh := newTestLifecycleManager(fs, &fakeCmdRunner{}, source, svc, noopScheduleManager{})
				if err := fresh.Install(context.Background(), "v2", false, noopProgress); !errors.Is(err, ErrMihomoAlreadyInstalled) {
					t.Fatalf("install=%v", err)
				}
				if string(fs.written[binaryPath]) != "binary-v1" || svc.running != running {
					t.Fatalf("binary=%q running=%v", fs.written[binaryPath], svc.running)
				}
				if _, ok := fs.written[lifecycleUpgradeTransactionFile]; ok {
					t.Fatal("journal retained after successful recovery")
				}
				if err := fresh.Upgrade(context.Background(), "v2", noopProgress); err != nil {
					t.Fatalf("retry upgrade=%v", err)
				}
				if string(fs.written[binaryPath]) != "binary-v2" || svc.running != running {
					t.Fatal("retry did not preserve intended state")
				}
			})
		}
	}
}

func TestInterruptedUpgradeRejectsWrongBackupAndPreservesJournal(t *testing.T) {
	transaction := upgradeTransaction{State: "in-progress", OldHash: lifecycleBinaryHash([]byte("original")), TempPath: binaryPath + ".tmp.v2", WasRunning: true}
	marker, _ := json.Marshal(transaction)
	fs := &fakeFileSystem{written: map[string][]byte{lifecycleUpgradeTransactionFile: marker, backupDir + "/mihomo.bak": []byte("unrelated stale backup")}}
	svc := &mockServiceManager{}
	m := newTestLifecycleManager(fs, &fakeCmdRunner{}, &fakeReleaseSource{}, svc, noopScheduleManager{})
	if err := m.Upgrade(context.Background(), "v2", noopProgress); err == nil {
		t.Fatal("accepted unrelated backup")
	}
	if _, ok := fs.written[lifecycleUpgradeTransactionFile]; !ok || svc.startCalls != 0 {
		t.Fatal("recovery evidence lost or wrong binary started")
	}
}

func TestCompletedUpgradeRecoveryDoesNotRollBack(t *testing.T) {
	transaction := upgradeTransaction{State: "completed", OldHash: lifecycleBinaryHash([]byte("old")), TempPath: binaryPath + ".tmp.v2", WasRunning: true}
	marker, _ := json.Marshal(transaction)
	fs := &fakeFileSystem{fileExists: map[string]bool{binaryPath: true}, written: map[string][]byte{binaryPath: []byte("new"), lifecycleUpgradeTransactionFile: marker}}
	svc := &mockServiceManager{running: true}
	m := newTestLifecycleManager(fs, &fakeCmdRunner{}, &fakeReleaseSource{}, svc, noopScheduleManager{})
	if err := m.Install(context.Background(), "v2", false, noopProgress); !errors.Is(err, ErrMihomoAlreadyInstalled) {
		t.Fatal(err)
	}
	if string(fs.written[binaryPath]) != "new" || svc.stopCalls != 0 || svc.startCalls != 0 {
		t.Fatal("committed upgrade rolled back")
	}
	if _, ok := fs.written[lifecycleUpgradeTransactionFile]; ok {
		t.Fatal("completed journal not cleaned")
	}
}

func TestInterruptedUpgradeFailedStopPreservesRecoveryFiles(t *testing.T) {
	transaction := upgradeTransaction{State: "in-progress", OldHash: lifecycleBinaryHash([]byte("original")), TempPath: binaryPath + ".tmp.v2", WasRunning: true}
	marker, _ := json.Marshal(transaction)
	fs := &fakeFileSystem{fileExists: map[string]bool{binaryPath: true}, written: map[string][]byte{binaryPath: []byte("replacement"), backupDir + "/mihomo.bak": []byte("original"), lifecycleUpgradeTransactionFile: marker}}
	stopErr := errors.New("cannot stop replacement")
	svc := &mockServiceManager{running: true, stopErr: stopErr}
	m := newTestLifecycleManager(fs, &fakeCmdRunner{}, &fakeReleaseSource{}, svc, noopScheduleManager{})
	if err := m.Install(context.Background(), "v2", false, noopProgress); !errors.Is(err, stopErr) {
		t.Fatalf("err=%v", err)
	}
	if string(fs.written[binaryPath]) != "replacement" || string(fs.written[backupDir+"/mihomo.bak"]) != "original" {
		t.Fatal("failed stop destroyed recovery files")
	}
	svc.stopErr = nil
	if err := m.Install(context.Background(), "v2", false, noopProgress); !errors.Is(err, ErrMihomoAlreadyInstalled) {
		t.Fatalf("retry=%v", err)
	}
	if string(fs.written[binaryPath]) != "original" || !svc.running {
		t.Fatal("retry recovery did not restore original")
	}
}
