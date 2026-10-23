//go:build !darwin && !linux && !windows

package installer

import "os"

// Unsupported development hosts do not participate in release installation.
// Keep the package buildable while retaining the release-target API.
func lockRunningFile(*os.File) error { return nil }

func unlockRunningFile(*os.File) error { return nil }
