//go:build !unix

package testutil

import "os"

// Unix signal scenarios are never selected by tests on other platforms.
func accountDefaultSignalFixture() { os.Exit(15) }
