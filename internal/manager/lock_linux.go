package manager

import (
	"errors"
	"os"
	"syscall"
)

func lockOperationFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}
func unlockOperationFile(file *os.File) error { return syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }
func operationLockBusy(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)
}
