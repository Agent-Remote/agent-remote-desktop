//go:build unix

package cliadapter

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

// CLI operations may own probe and credential-tool subprocess groups. Give the
// SIGTERM handler time to unwind and close them before the kill fallback.
func configureDiagnosticCancellation(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 2 * time.Second
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
}
