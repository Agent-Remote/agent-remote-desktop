// Command terminal-fixture is a tiny native Agent stand-in used by the
// Windows desktop tests. It stays attached to the ConPTY until Ctrl+C and
// then exits with the status the shutdown test asserts.
package main

import (
	"fmt"
	"os"
	"os/signal"
)

func main() {
	fmt.Fprintln(os.Stdout, "ready")
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	// ConPTY normally turns ETX into os.Interrupt, but hosted Windows runners
	// can deliver the byte through the input stream without raising the signal.
	// Consume that path as well so the fixture always terminates on a graceful
	// terminal stop and releases its executable before TempDir cleanup.
	go func() {
		buf := make([]byte, 32)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				return
			}
			for _, b := range buf[:n] {
				if b == 3 {
					os.Exit(42)
				}
			}
		}
	}()
	<-signals
	os.Exit(42)
}
