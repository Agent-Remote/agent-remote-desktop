//go:build darwin || linux

package terminalhost

import (
	"errors"
	"os/exec"
	"syscall"

	"github.com/charmbracelet/x/xpty"
)

type processGroup struct{}

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
}
func (*processGroup) attach(*exec.Cmd) error { return nil }
func (*processGroup) close()                 {}
func (*processGroup) signal(cmd *exec.Cmd, _ xpty.Pty, force bool) error {
	signal := syscall.SIGTERM
	if force {
		signal = syscall.SIGKILL
	}
	// Give the process that owns the PTY the first chance to run its normal
	// termination trap. Signalling the whole process group immediately can
	// kill a shell's foreground helper (for example `sleep`) before the shell
	// handles SIGTERM, which makes a graceful exit look like signal -1 and
	// loses the final exit status. Host.Stop force-cleans the group after the
	// leader has exited so descendants cannot be left behind.
	pid := cmd.Process.Pid
	if force {
		pid = -pid
	}
	err := syscall.Kill(pid, signal)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
func environmentKeyEqual(a, b string) bool { return a == b }
