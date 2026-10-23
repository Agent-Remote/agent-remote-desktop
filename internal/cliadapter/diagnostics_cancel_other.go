//go:build !unix

package cliadapter

import "os/exec"

// On Windows the CLI owns kill-on-close jobs for probes, so terminating the CLI
// also closes those jobs and stops their descendants.
func configureDiagnosticCancellation(cmd *exec.Cmd) {}
