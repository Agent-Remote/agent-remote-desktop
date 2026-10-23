// workbench-probe exercises the real application, UI events and managed PTY
// against a temporary executable. It never launches an installed Agent.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo"
)

type event struct {
	Kind      string
	PID       int
	Args      []string
	Input     string
	Home      string
	Directory string
}

func main() {
	if testutil.ControlHelper() {
		return
	}
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if name == "codex" || name == "fclaude" || (name == "agent-remote" && len(os.Args) > 3 && os.Args[1] == "account" && os.Args[2] == "attach") {
		agent()
		return
	}
	root, err := os.MkdirTemp("", "desktop-workbench-probe-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(root)
	if err = prepareAgent(root); err != nil {
		panic(err)
	}
	mygo.App.SetName("Desktop Workbench Probe")
	mygo.App.OnWindowAllClosed(func() {})
	result := make(chan error, 1)
	mygo.App.WhenReady(func() {
		go func() {
			err := runProbe(root)
			if err == nil {
				err = runRemoteProbe(root)
			}
			if err == nil {
				err = runAccountBindingProbe(root)
			}
			if err == nil {
				err = runRemoteScopeProbe(root)
			}
			result <- err
			mygo.App.Quit()
		}()
	})
	runErr := mygo.App.Run()
	probeErr := <-result
	if probeErr != nil {
		err = probeErr
	} else {
		err = runErr
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		_ = os.RemoveAll(root)
		os.Exit(1)
	}
	fmt.Println("workbench launch, focus, drag, shutdown, exact remote context/reuse/history replacement, exact binding attachment/reuse and history refresh: true")
}

func prepareAgent(root string) error {
	dir := filepath.Join(root, "bin")
	if err := os.Mkdir(dir, 0700); err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	source, err := os.Open(executable)
	if err != nil {
		return err
	}
	defer source.Close()
	for _, name := range []string{"codex", "fclaude", "agent-remote"} {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		if _, err := source.Seek(0, io.SeekStart); err != nil {
			return err
		}
		destination, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(destination, source)
		if err := errors.Join(copyErr, destination.Close()); err != nil {
			return err
		}
	}
	for key, value := range map[string]string{"PATH": dir + string(os.PathListSeparator) + os.Getenv("PATH"), "MYGO_LOCALE": "en", "DESKTOP_TEST_CLI_SCENARIO": "", "DESKTOP_TEST_CLI_LOG": "", "DESKTOP_WORKBENCH_EVENTS": filepath.Join(root, "events.jsonl")} {
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return nil
}

func agent() {
	// A failed parent probe cannot leave this temporary helper running indefinitely.
	time.AfterFunc(time.Minute, func() { os.Exit(0) })
	write := func(kind, input string) {
		f, err := os.OpenFile(os.Getenv("DESKTOP_WORKBENCH_EVENTS"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			panic(err)
		}
		dir, _ := os.Getwd()
		if err = json.NewEncoder(f).Encode(event{Kind: kind, PID: os.Getpid(), Args: os.Args[1:], Input: input, Home: os.Getenv("AGENT_REMOTE_HOME"), Directory: dir}); err != nil {
			panic(err)
		}
		_ = f.Close()
	}
	write("started", "")
	fmt.Println("Probe Agent ready")
	reader := bufio.NewScanner(os.Stdin)
	for reader.Scan() {
		write("input", reader.Text())
		fmt.Println("received:", reader.Text())
	}
}

func events(root string) []event {
	f, err := os.Open(filepath.Join(root, "events.jsonl"))
	if err != nil {
		return nil
	}
	defer f.Close()
	var result []event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e event
		if json.Unmarshal(scanner.Bytes(), &e) == nil {
			result = append(result, e)
		}
	}
	return result
}

func closeApp(a *app.App) error {
	var err error
	mygo.RunOnMain(func() { err = a.Close() })
	return err
}
