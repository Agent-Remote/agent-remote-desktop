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
	<-signals
	os.Exit(42)
}
