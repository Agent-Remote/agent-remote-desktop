// native-probe verifies a native window and managed PTY survive hide/show.
// It uses a temporary directory and a harmless shell, never user Agent state.
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/terminalhost"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func main() {
	// The probe must never overwrite the user's persisted language preference.
	if err := os.Setenv("MYGO_LOCALE", "en"); err != nil {
		panic(err)
	}
	mygo.App.SetName("Agent Remote Native Probe")
	mygo.App.OnWindowAllClosed(func() {})
	results := make(chan error, 1)
	var quitting atomic.Bool
	finish := func(err error) {
		quitting.Store(true)
		results <- err
		mygo.App.Quit()
	}
	mygo.App.WhenReady(func() {
		cmd := []string{"/bin/sh", "-c", "printf 'native-ready\\n'; read answer; printf 'native-result:%s\\n' \"$answer\""}
		if runtime.GOOS == "windows" {
			cmd = []string{"cmd.exe", "/Q", "/C", "echo native-ready & set /p answer= & call echo native-result:%%answer%%"}
		}
		dir, err := os.MkdirTemp("", "agent-remote-native-probe-")
		if err != nil {
			finish(err)
			return
		}
		host, err := terminalhost.New(session.Command{Argv: cmd, Directory: dir})
		if err != nil {
			os.RemoveAll(dir)
			finish(err)
			return
		}
		term := host.Terminal
		win := mygo.NewWindow(mygo.WindowOptions{Title: locales.Workbench(), Width: 900, Height: 600, Content: ui.View(func(c *ui.Context) {
			ui.Column(c).Fill().Children(func() {
				ui.Text(c, locales.Workbench())
				terminal.View(c, term).Grow(1).FillWidth()
			})
		})})
		var languageEvents atomic.Int32
		var closeEvents atomic.Int32
		// Exercise the same close contract used by the real workbench: a
		// window close hides the window while the process and its PTY remain
		// alive. Calling Close (instead of Hide) makes this a native close
		// button probe on every supported desktop backend.
		win.OnClose(func(e *mygo.CloseEvent) {
			closeEvents.Add(1)
			if quitting.Load() {
				return
			}
			e.PreventDefault()
			win.Hide()
		})
		off := mygo.App.OnLocaleChanged(func(string) {
			languageEvents.Add(1)
			win.SetTitle(locales.Workbench())
		})
		go func() {
			defer os.RemoveAll(dir)
			defer off()
			var failure error
			deadline := time.Now().Add(10 * time.Second)
			for !strings.Contains(term.Text(), "native-ready") && time.Now().Before(deadline) {
				time.Sleep(25 * time.Millisecond)
			}
			// A real window resize must reach the native PTY/ConPTY through
			// terminal.View. Keep this check independent of a fixed cell size:
			// native font metrics differ across platforms, but the cell count
			// must change after the content area grows.
			initialCols, initialRows, sizeErr := host.Size()
			if sizeErr != nil {
				failure = sizeErr
			} else {
				mygo.RunOnMain(func() { win.SetSize(1100, 700) })
				deadline = time.Now().Add(3 * time.Second)
				for time.Now().Before(deadline) {
					cols, rows, err := host.Size()
					if err == nil && (cols != initialCols || rows != initialRows) && cols > 0 && rows > 0 {
						break
					}
					time.Sleep(25 * time.Millisecond)
				}
				cols, rows, err := host.Size()
				if err != nil || (cols == initialCols && rows == initialRows) {
					failure = fmt.Errorf("native window resize did not reach PTY (initial=%dx%d current=%dx%d err=%v)", initialCols, initialRows, cols, rows, err)
				}
			}
			for _, choice := range []struct{ tag, title string }{{"zh-Hans", "工作台"}, {"en", "Workbench"}} {
				changed := make(chan error, 1)
				mygo.RunOnMain(func() {
					mygo.App.SetLocale(choice.tag)
					if mygo.App.Locale() != choice.tag || locales.Workbench() != choice.title {
						changed <- fmt.Errorf("locale change did not reach generated catalog: %s", choice.tag)
					} else {
						changed <- nil
					}
				})
				select {
				case err := <-changed:
					if err != nil {
						failure = err
					}
				case <-time.After(5 * time.Second):
					failure = fmt.Errorf("locale change timed out")
				}
			}
			if languageEvents.Load() < 2 {
				failure = fmt.Errorf("locale change listeners were not notified")
			}
			win.Close()
			if closeEvents.Load() != 1 || win.IsVisible() {
				failure = fmt.Errorf("native close did not hide window (events=%d visible=%t)", closeEvents.Load(), win.IsVisible())
			}
			term.Send([]byte("hidden-window-alive\r"))
			select {
			case <-host.Done():
			case <-time.After(10 * time.Second):
				failure = fmt.Errorf("hidden PTY timed out")
			}
			deadline = time.Now().Add(time.Second)
			for !strings.Contains(term.Text(), "native-result:hidden-window-alive") && time.Now().Before(deadline) {
				time.Sleep(20 * time.Millisecond)
			}
			win.Show()
			win.Focus()
			if !strings.Contains(term.Text(), "native-result:hidden-window-alive") {
				failure = fmt.Errorf("terminal output lost across hide/show")
			}
			if host.ExitCode() != 0 {
				failure = fmt.Errorf("PTY exit code: %d", host.ExitCode())
			}
			fmt.Println("native locale switch and window hide/show:", failure == nil, "PTY exit:", host.ExitCode())
			_ = host.Stop(true)
			if err := host.Close(); err != nil && failure == nil {
				failure = err
			}
			finish(failure)
		}()
	})
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	select {
	case err := <-results:
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "probe interrupted")
		os.Exit(1)
	}
}
