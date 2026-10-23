//go:build windows

package terminalhost

import (
	"errors"
	"os/exec"
	"strings"
	"unsafe"

	"github.com/charmbracelet/x/xpty"
	"golang.org/x/sys/windows"
)

type processGroup struct{ job windows.Handle }

func configureProcess(*exec.Cmd) {}
func (g *processGroup) attach(cmd *exec.Cmd) error {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return err
	}
	p, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		windows.CloseHandle(job)
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return nil
		}
		return err
	}
	defer windows.CloseHandle(p)
	if err = windows.AssignProcessToJobObject(job, p); err != nil {
		windows.CloseHandle(job)
		// A desktop launched from a CI runner, service host, or enterprise
		// launcher may already be inside a Job Object. Windows normally refuses
		// nested assignment; keep the ConPTY usable and let signal(force) fall
		// back to terminating the direct process in that environment. The
		// parent's job still owns descendants when it has a kill-on-close policy.
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return nil
		}
		return err
	}
	g.job = job
	return nil
}
func (g *processGroup) close() {
	if g.job != 0 {
		windows.CloseHandle(g.job)
		g.job = 0
	}
}
func (g *processGroup) signal(cmd *exec.Cmd, p xpty.Pty, force bool) error {
	if force {
		if g.job != 0 {
			return windows.TerminateJobObject(g.job, 1)
		}
		// The process can be hosted by an existing Job Object that rejects
		// nested assignment. Closing that external job is outside the
		// desktop's authority, so terminate the direct child as a bounded
		// fallback. ConPTY remains responsible for the stream handle.
		if cmd == nil || cmd.Process == nil {
			return windows.ERROR_INVALID_HANDLE
		}
		return cmd.Process.Kill()
	}
	// ConPTY delivers ETX as the terminal's Ctrl+C; force is a separate user decision.
	_, err := p.Write([]byte{3})
	return err
}
func environmentKeyEqual(a, b string) bool { return strings.EqualFold(a, b) }
