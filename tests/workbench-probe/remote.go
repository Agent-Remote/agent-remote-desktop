package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func runRemoteProbe(root string) (result error) {
	previous := os.Getenv("DESKTOP_TEST_REMOTE_DEVICE")
	os.Setenv("DESKTOP_TEST_REMOTE_DEVICE", testutil.RemoteAttachDeviceID)
	defer os.Setenv("DESKTOP_TEST_REMOTE_DEVICE", previous)
	const id = "11111111-1111-4111-8111-111111111111"
	home := filepath.Join(root, "home")
	var application *app.App
	var err error
	d := driver{root: root}
	mygo.RunOnMain(func() {
		application, err = app.New(home)
		if err == nil {
			d.view = ui.NewTester(application.View, 1366, 768)
		}
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := closeApp(application); result == nil {
			result = err
		}
	}()
	open := func() error {
		// Use the stable command shortcut so duplicate tile title/subtitle labels
		// cannot make a headless click target the non-clickable subtitle.
		if err := d.key(ui.Cmd|ui.Shift, ui.KeyR); err != nil {
			return err
		}
		if err := d.settle(); err != nil {
			return err
		}
		err := d.wait(func() bool {
			ready := false
			_ = d.action(func(t *ui.Tester) error {
				t.Frame()
				ready = t.HasText(id) && !t.HasText(locales.LoadingRemote()) && t.HasText(locales.ConnectRemoteSession())
				return nil
			})
			return ready
		})
		if err != nil {
			var texts []string
			_ = d.action(func(t *ui.Tester) error { t.Frame(); texts = t.Texts(); return nil })
			return fmt.Errorf("remote list did not become ready: %w; texts: %q", err, texts)
		}
		return nil
	}
	if err = open(); err != nil {
		return fmt.Errorf("remote open: %w", err)
	}
	if err = d.click(locales.ConnectRemoteSession()); err != nil {
		return err
	}
	if err = d.action(func(t *ui.Tester) error {
		if !t.HasText(locales.RemoteAttachConfirmation()) {
			return fmt.Errorf("remote attachment skipped consent; texts: %q", t.Texts())
		}
		return nil
	}); err != nil {
		return err
	}
	if err = d.click(locales.ConfirmRemoteConnection()); err != nil {
		return err
	}
	if err = d.waitStarted(5); err != nil {
		return fmt.Errorf("confirmed remote launch: %w", err)
	}
	if err = d.waitReady(); err != nil {
		return fmt.Errorf("confirmed remote terminal ready: %w", err)
	}
	// The attach confirmation closes asynchronously; explicitly focus the new
	// pane chip before typing so headless/native backends agree on ownership.
	if err = d.click("fclaude " + id); err != nil {
		return fmt.Errorf("focus remote pane: %w", err)
	}
	if err = d.settle(); err != nil {
		return err
	}
	args := []string{"attach", id, "--expected-server", "https://test.invalid", "--expected-device", testutil.RemoteAttachDeviceID}
	if err = d.input("remote-input", args); err != nil {
		var texts []string
		_ = d.action(func(t *ui.Tester) error { t.Frame(); texts = t.Texts(); return nil })
		return fmt.Errorf("remote input: %w; texts: %q", err, texts)
	}
	for _, e := range events(root) {
		if e.Kind == "started" && reflect.DeepEqual(e.Args, args) {
			actual, _ := filepath.EvalSymlinks(e.Directory)
			expected, _ := filepath.EvalSymlinks(root)
			if e.Home != home || actual != expected {
				return fmt.Errorf("attachment lost selected configuration or project")
			}
		}
	}
	if err = open(); err != nil {
		return err
	}
	if err = d.click(locales.ConnectRemoteSession()); err != nil {
		return err
	}
	if err = d.input("same-remote-connection", args); err != nil {
		return err
	}
	count := 0
	for _, e := range events(root) {
		if e.Kind == "started" {
			count++
		}
	}
	if count != 5 {
		return fmt.Errorf("remote connection duplicated: %d starts", count)
	}
	return nil
}
