// platform-probe emits a bounded, machine-readable report for the current
// host. It deliberately never sets GOOS/GOARCH and never claims cross-build
// output is runtime evidence.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Agent-Remote/agent-remote-desktop/internal/platform"
)

func main() {
	report := platform.ProbeRuntime()
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, "platform probe:", err)
		os.Exit(1)
	}
	if !report.Supported {
		fmt.Fprintln(os.Stderr, "platform probe: required host runtime checks did not pass")
		os.Exit(2)
	}
}
