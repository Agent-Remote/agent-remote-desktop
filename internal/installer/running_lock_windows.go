//go:build windows

package installer

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func lockRunningFile(file *os.File) error {
	var overlapped windows.Overlapped
	err := windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
	if err != nil {
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
			return fmt.Errorf("%w: another desktop process holds the running lock", ErrInstallBusy)
		}
		return fmt.Errorf("%w: lock: %v", ErrInstallBusy, err)
	}
	return nil
}

func unlockRunningFile(file *os.File) error {
	var overlapped windows.Overlapped
	return windows.UnlockFileEx(windows.Handle(file.Fd()), 0, 1, 0, &overlapped)
}
