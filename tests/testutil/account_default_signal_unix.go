//go:build unix

package testutil

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

// This subprocess fixture records receipt of the actual OS signal before
// returning. SIGKILL cannot execute this cleanup path or produce the evidence.
func accountDefaultSignalFixture() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM)
	defer signal.Stop(signals)
	if os.WriteFile(os.Getenv("DESKTOP_TEST_DEFAULT_SIGNAL_READY"), []byte("ready"), 0600) != nil {
		os.Exit(11)
	}
	select {
	case received := <-signals:
		if received != syscall.SIGTERM {
			os.Exit(12)
		}
		// Exercise a short cleanup interval after signal receipt, before process exit.
		time.Sleep(30 * time.Millisecond)
		if os.WriteFile(os.Getenv("DESKTOP_TEST_DEFAULT_SIGNAL_CLEANUP"), []byte("SIGTERM cleanup completed"), 0600) != nil {
			os.Exit(13)
		}
		os.Exit(0)
	case <-time.After(15 * time.Second):
		os.Exit(14)
	}
}
