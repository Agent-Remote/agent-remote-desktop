package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func runAccountBindingProbe(root string) (result error) {
	const accountID = "11111111-1111-4111-8111-111111111111"
	const bindingID = "bind-111111111111-aabbccddeeff"
	if err := os.Setenv("DESKTOP_TEST_CLI_SCENARIO", "account-terminal-ready"); err != nil {
		return err
	}
	defer os.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	startedCount := func() int {
		count := 0
		for _, e := range events(root) {
			if e.Kind == "started" {
				count++
			}
		}
		return count
	}
	home := filepath.Join(root, "home")
	var application *app.App
	var err error
	d := driver{root: root}
	openApp := func() error {
		mygo.RunOnMain(func() {
			application, err = app.New(home)
			if err == nil {
				d.view = ui.NewTester(application.View, 1366, 768)
			}
		})
		return err
	}
	if err = openApp(); err != nil {
		return err
	}
	defer func() {
		if application != nil {
			if err := closeApp(application); result == nil {
				result = err
			}
		}
	}()
	waitStatus := func() error {
		return d.wait(func() bool {
			ready := false
			_ = d.action(func(t *ui.Tester) error {
				t.Frame()
				ready = t.HasText(locales.AccountBindingSession(bindingID)) && !t.HasText(locales.LoadingRemote())
				return nil
			})
			return ready
		})
	}
	open := func() error {
		if err := d.click(locales.RemoteManagement()); err != nil {
			return fmt.Errorf("click remote management: %w", err)
		}
		if err := d.wait(func() bool {
			ready := false
			_ = d.action(func(t *ui.Tester) error {
				t.Frame()
				ready = t.HasText(locales.RemoteAccounts()) && !t.HasText(locales.LoadingRemote())
				return nil
			})
			return ready
		}); err != nil {
			return fmt.Errorf("open account remote management: %w", err)
		}
		if err := d.click(locales.RemoteAccounts()); err != nil {
			return fmt.Errorf("click remote accounts: %w", err)
		}
		if err := d.click(locales.AccountDetailsTitle()); err != nil {
			return fmt.Errorf("click account details: %w", err)
		}
		if err := waitStatus(); err != nil {
			return fmt.Errorf("binding status: %w", err)
		}
		if err := d.action(func(t *ui.Tester) error { t.Scroll(900, 400, 0, 10000); return nil }); err != nil {
			return fmt.Errorf("scroll account details: %w", err)
		}
		if err := d.click(locales.AccountBindingConnectDetailsLabel()); err != nil {
			return fmt.Errorf("click binding connect: %w", err)
		}
		return nil
	}
	if err = open(); err != nil {
		return fmt.Errorf("open initial binding: %w", err)
	}
	if err = d.action(func(t *ui.Tester) error {
		for _, text := range []string{locales.AccountExactID(accountID), locales.AccountBindingSession(bindingID), "https://test.invalid", "device-1", root, locales.AccountBindingConnectHelp()} {
			if !t.HasText(text) {
				return fmt.Errorf("binding confirmation missing %q", text)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if err = d.waitStarted(5); err != nil {
		return fmt.Errorf("binding review started terminal: %w", err)
	}
	if err = d.click(locales.Cancel()); err != nil {
		return err
	}
	if err = d.action(func(t *ui.Tester) error { t.Scroll(900, 400, 0, 10000); return nil }); err != nil {
		return err
	}
	if err = d.click(locales.AccountBindingConnect()); err != nil {
		return err
	}
	if err = d.action(func(t *ui.Tester) error {
		t.Frame()
		return t.Click(locales.AccountBindingConfirmLabel())
	}); err != nil {
		return err
	}
	if err = d.waitStarted(6); err != nil {
		var texts []string
		_ = d.action(func(t *ui.Tester) error { t.Frame(); texts = t.Texts(); return nil })
		return fmt.Errorf("confirmed binding launch: %w; UI: %v", err, texts)
	}
	if err = d.waitReady(); err != nil {
		return fmt.Errorf("confirmed binding terminal ready: %w", err)
	}
	args := []string{"account", "attach", accountID, "--binding-session", bindingID, "--expected-server", "https://test.invalid", "--expected-device", "device-1"}
	if err = d.input("binding-login-input", args); err != nil {
		return fmt.Errorf("binding terminal input: %w", err)
	}
	for _, e := range events(root) {
		if e.Kind == "started" && reflect.DeepEqual(e.Args, args) {
			actual, _ := filepath.EvalSymlinks(e.Directory)
			expected, _ := filepath.EvalSymlinks(root)
			if actual != expected || e.Home != home {
				return fmt.Errorf("binding lost selected home/project")
			}
		}
	}
	if err = open(); err != nil {
		return fmt.Errorf("reopen binding: %w", err)
	}
	beforeReuse := startedCount()
	// Opening the account details view is asynchronous. Wait for the fresh
	// details frame before clicking its confirmation button so the click cannot
	// land on the stale workbench frame from the previous view.
	if err = d.wait(func() bool {
		ready := false
		_ = d.action(func(t *ui.Tester) error {
			t.Frame()
			ready = t.HasText(locales.AccountBindingConnectHelp())
			return nil
		})
		return ready
	}); err != nil {
		return fmt.Errorf("binding reuse review: %w", err)
	}
	if err = d.action(func(t *ui.Tester) error {
		t.Frame()
		return t.Click(locales.AccountBindingConfirmLabel())
	}); err != nil {
		return err
	}
	if err = d.input("same-binding-focused", args); err != nil {
		return err
	}
	if afterReuse := startedCount(); afterReuse != beforeReuse {
		return fmt.Errorf("binding reuse duplicated process: started count %d -> %d", beforeReuse, afterReuse)
	}
	if err = closeApp(application); err != nil {
		return err
	}
	application = nil
	if err = openApp(); err != nil {
		return err
	}
	if err = d.action(func(t *ui.Tester) error {
		t.Frame()
		if !t.HasText(locales.AccountBindingReconnectHelp()) {
			return fmt.Errorf("binding history omitted fresh-status requirement")
		}
		return nil
	}); err != nil {
		return err
	}
	if err = d.click(locales.AccountBindingReviewStatus()); err != nil {
		return err
	}
	if err = waitStatus(); err != nil {
		return fmt.Errorf("binding history status: %w", err)
	}
	// A saved binding history is metadata only.  Fresh status is required
	// before opening the review, and the terminal still needs a new explicit
	// confirmation; reopening the application must never resume a credentialed
	// binding automatically.
	if err = d.action(func(t *ui.Tester) error {
		t.Frame()
		for _, text := range []string{locales.AccountBindingConnectDetailsLabel()} {
			if !t.HasText(text) {
				return fmt.Errorf("binding history missing %q", text)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if err = d.action(func(t *ui.Tester) error {
		t.Scroll(900, 400, 0, 10000)
		t.Frame()
		return t.Click(locales.AccountBindingConnectDetailsLabel())
	}); err != nil {
		return err
	}
	if err = d.wait(func() bool {
		ready := false
		_ = d.action(func(t *ui.Tester) error {
			t.Frame()
			ready = t.HasText(locales.AccountBindingConnectHelp())
			return nil
		})
		return ready
	}); err != nil {
		return fmt.Errorf("binding history skipped fresh confirmation: %w", err)
	}
	if err = d.action(func(t *ui.Tester) error {
		t.Frame()
		return t.Click(locales.AccountBindingConfirmLabel())
	}); err != nil {
		return err
	}
	if err = d.waitStarted(7); err != nil {
		return fmt.Errorf("confirmed history reconnect: %w", err)
	}
	return d.waitReady()
}
