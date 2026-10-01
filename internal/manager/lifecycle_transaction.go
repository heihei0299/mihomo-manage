package manager

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var lifecycleUpgradeTransactionFile = filepath.Join(stateDir, "upgrade-transaction.json")

// The journal is written before stopping the service. An in-progress operation
// restores the binary identified by OldHash and the previous running state;
// completed means the new binary/service state is committed, with cleanup only.
// Hash checks distinguish an untouched/restored binary from a replacement even
// when a process dies between rename and the next persistent state write.
type upgradeTransaction struct {
	State      string `json:"state"`
	OldHash    string `json:"old_hash"`
	WasRunning bool   `json:"was_running"`
	TempPath   string `json:"temp_path"`
}

func lifecycleBinaryHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (m *lifecycleManager) writeUpgradeTransaction(transaction upgradeTransaction) error {
	data, err := json.Marshal(transaction)
	if err != nil {
		return err
	}
	if err := m.fs.MkdirAll(stateDir, dirPermPrivate); err != nil {
		return err
	}
	if err := m.fs.Chmod(stateDir, dirPermPrivate); err != nil {
		return err
	}
	tmp := lifecycleUpgradeTransactionFile + ".tmp"
	if err := m.fs.WriteFile(tmp, data, filePermPrivateRW); err != nil {
		return errors.Join(err, m.fs.RemoveAll(tmp))
	}
	if err := m.fs.Rename(tmp, lifecycleUpgradeTransactionFile); err != nil {
		return errors.Join(err, m.fs.RemoveAll(tmp))
	}
	return nil
}

func (m *lifecycleManager) cleanupUpgradeTransaction(transaction upgradeTransaction) error {
	if err := m.fs.RemoveAll(transaction.TempPath); err != nil {
		return fmt.Errorf("cleanup upgrade staging: %w", err)
	}
	return m.fs.RemoveAll(lifecycleUpgradeTransactionFile)
}

func (m *lifecycleManager) completeUpgradeTransaction(transaction upgradeTransaction) error {
	transaction.State = "completed"
	if err := m.writeUpgradeTransaction(transaction); err != nil {
		return fmt.Errorf("recording completed upgrade: %w", err)
	}
	return m.cleanupUpgradeTransaction(transaction)
}

func (m *lifecycleManager) recoverUpgradeTransaction(ctx context.Context) error {
	data, err := m.fs.ReadFile(lifecycleUpgradeTransactionFile)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var transaction upgradeTransaction
	if err := json.Unmarshal(data, &transaction); err != nil {
		return fmt.Errorf("decoding upgrade transaction: %w", err)
	}
	if transaction.TempPath == "" {
		return errors.New("upgrade transaction is missing staging path")
	}
	if transaction.State == "completed" {
		return m.cleanupUpgradeTransaction(transaction)
	}
	if transaction.State != "in-progress" || len(transaction.OldHash) != 64 {
		return errors.New("invalid upgrade transaction")
	}
	rollbackCtx := context.WithoutCancel(ctx)
	current, readErr := m.fs.ReadFile(binaryPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("reading interrupted binary: %w", readErr)
	}
	if readErr != nil || lifecycleBinaryHash(current) != transaction.OldHash {
		backup := backupDir + "/mihomo.bak"
		previous, err := m.fs.ReadFile(backup)
		if err != nil {
			return fmt.Errorf("reading interrupted upgrade backup: %w", err)
		}
		if lifecycleBinaryHash(previous) != transaction.OldHash {
			return errors.New("interrupted upgrade backup does not match original binary; manual recovery required")
		}
		if err := m.restoreBinary(rollbackCtx, backup, transaction.TempPath, transaction.WasRunning, true); err != nil {
			return fmt.Errorf("recovering interrupted binary: %w", err)
		}
	} else {
		running, err := m.svcMgr.IsRunning(rollbackCtx, serviceName)
		if err != nil {
			return err
		}
		if running != transaction.WasRunning {
			if transaction.WasRunning {
				err = m.svcMgr.Start(rollbackCtx, serviceName)
			} else {
				err = m.svcMgr.Stop(rollbackCtx, serviceName)
			}
			if err != nil {
				return fmt.Errorf("restoring interrupted service state: %w", err)
			}
			running, err = m.svcMgr.IsRunning(rollbackCtx, serviceName)
			if err != nil {
				return err
			}
			if running != transaction.WasRunning {
				return errors.New("interrupted service did not return to its previous state")
			}
		}
	}
	return m.cleanupUpgradeTransaction(transaction)
}
