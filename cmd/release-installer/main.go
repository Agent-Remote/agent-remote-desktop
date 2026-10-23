// Command release-installer validates and atomically installs a local desktop
// release directory. It is shipped beside each desktop artifact so upgrades
// do not depend on a source checkout.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/Agent-Remote/agent-remote-desktop/internal/installer"
)

func main() {
	release := flag.String("release", "", "validated release directory")
	destination := flag.String("destination", "", "absolute installation directory")
	platform := flag.String("platform", runtime.GOOS, "target operating system (darwin, windows, linux)")
	arch := flag.String("arch", runtime.GOARCH, "target architecture (amd64 or arm64)")
	rollback := flag.Bool("rollback", false, "restore the previous installation")
	status := flag.Bool("status", false, "print read-only installation status")
	allowDowngrade := flag.Bool("allow-downgrade", false, "allow an explicitly requested version downgrade")
	flag.Parse()
	if flag.NArg() != 0 {
		fail("unexpected positional arguments")
	}
	if *destination == "" {
		fail("--destination is required")
	}
	if *status {
		if *rollback || *release != "" || *allowDowngrade {
			fail("--status cannot be combined with --release, --rollback, or --allow-downgrade")
		}
		state, err := installer.Inspect(*destination)
		if err != nil {
			fail(err.Error())
		}
		if err := json.NewEncoder(os.Stdout).Encode(state); err != nil {
			fail(fmt.Sprintf("write status: %v", err))
		}
		return
	}
	var receipt installer.Receipt
	var err error
	if *rollback {
		receipt, err = installer.Rollback(*destination)
	} else {
		if *release == "" {
			fail("--release is required unless --rollback is used")
		}
		receipt, err = installer.Install(*release, *platform, *arch, *destination, *allowDowngrade)
	}
	if err != nil {
		fail(err.Error())
	}
	if err := json.NewEncoder(os.Stdout).Encode(receipt); err != nil {
		fail(fmt.Sprintf("write receipt: %v", err))
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(2)
}
