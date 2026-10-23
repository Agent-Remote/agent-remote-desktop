package testutil

import (
	"os"
	"time"
)

// waitControlBarrier holds one fixture response until a test releases it or
// cancellation kills the CLI subprocess. It never blocks unrelated operations.
func waitControlBarrier(releaseEnv, startedEnv string) {
	release := os.Getenv(releaseEnv)
	if release == "" {
		return
	}
	if path := os.Getenv(startedEnv); path != "" {
		if os.WriteFile(path, nil, 0600) != nil {
			os.Exit(7)
		}
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(release); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	os.Exit(8)
}
