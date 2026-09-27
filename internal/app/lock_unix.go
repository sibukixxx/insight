//go:build !windows

package app

import (
	"errors"
	"os"
	"syscall"
)

var errLockHeld = errors.New("lock held")

func tryLock(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return errLockHeld
	}
	return err
}

func unlock(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
