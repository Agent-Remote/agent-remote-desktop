//go:build darwin || linux

package installer

import "os"

// syncDirectory persists directory entries after an atomic rename. The
// installer already fsyncs each staged file; syncing the parent closes the
// crash window where a successful rename could otherwise disappear after
// power loss on Unix filesystems.
func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	return func() error {
		syncErr := dir.Sync()
		closeErr := dir.Close()
		if syncErr != nil {
			return syncErr
		}
		return closeErr
	}()
}
