package installer

import (
	"os"
	"path/filepath"
)

// RunningLock is held by an installed desktop process for its whole lifetime.
// The lock is kept beside the installation tree so an installer can replace
// the tree atomically while still refusing to race a running process.
type RunningLock struct {
	file *os.File
}

// Close releases the lock and closes its descriptor. The lock file itself is
// intentionally retained; it is an installation identity marker and can be
// safely reused after a crash because the OS releases the advisory lock.
func (l *RunningLock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := unlockRunningFile(l.file)
	closeErr := l.file.Close()
	l.file = nil
	if err != nil {
		return err
	}
	return closeErr
}

// AcquireRunningLock obtains the exclusive lock associated with destination.
// It creates only the sibling lock marker and never creates or modifies the
// installation directory itself.
func AcquireRunningLock(destination string) (*RunningLock, error) {
	destination, err := absolutePath(destination)
	if err != nil {
		return nil, err
	}
	if err := ensureNoSymlinkAncestors(destination); err != nil {
		return nil, err
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(runningLockPath(destination), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := lockRunningFile(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return &RunningLock{file: file}, nil
}

func acquireMutationLock(destination string) (*RunningLock, error) {
	lock, err := AcquireRunningLock(destination)
	if err != nil {
		return nil, err
	}
	return lock, nil
}

// AcquireRunningLockForExecutable looks for the install sidecar while walking
// from an executable through its parent directories. Development builds have
// no sidecar and therefore do not create a lock.
func AcquireRunningLockForExecutable(executable string) (*RunningLock, error) {
	path, err := filepath.Abs(executable)
	if err != nil {
		return nil, err
	}
	if info, statErr := os.Stat(path); statErr == nil && !info.IsDir() {
		path = filepath.Dir(path)
	} else {
		path = filepath.Dir(path)
	}
	for {
		if _, statErr := os.Stat(statePath(path)); statErr == nil {
			return AcquireRunningLock(path)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return nil, nil
		}
		path = parent
	}
}

func runningLockPath(destination string) string { return destination + ".running.lock" }
