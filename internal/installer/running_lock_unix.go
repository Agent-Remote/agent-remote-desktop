//go:build darwin || linux

package installer

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

func lockRunningFile(file *os.File) error {
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return fmt.Errorf("%w: another desktop process holds the running lock", ErrInstallBusy)
		}
		return fmt.Errorf("%w: lock: %v", ErrInstallBusy, err)
	}
	return nil
}

func unlockRunningFile(file *os.File) error {
	if err := unix.Flock(int(file.Fd()), unix.LOCK_UN); err != nil {
		return err
	}
	return nil
}
