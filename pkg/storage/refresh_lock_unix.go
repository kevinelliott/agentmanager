//go:build !windows

package storage

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func tryRefreshLock(file *os.File) (bool, error) {
	err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB) //nolint:gosec // G115: OS file descriptors are bounded by the platform int range.
	if errors.Is(err, unix.EWOULDBLOCK) {
		return false, nil
	}
	return err == nil, err
}

func unlockRefresh(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN) //nolint:gosec // G115: OS file descriptors are bounded by the platform int range.
}
