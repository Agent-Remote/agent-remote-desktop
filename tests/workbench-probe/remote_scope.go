package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// runRemoteScopeProbe verifies real PTY launches, history replacement and live
// connection reuse across identical session UUIDs in distinct public contexts.
func runRemoteScopeProbe(root string) (result error) {
	const id = "11111111-1111-4111-8111-111111111111"
	for _, key := range []string{"DESKTOP_TEST_REMOTE_DEVICE", "DESKTOP_TEST_REMOTE_SERVER"} {
		previous := os.Getenv(key)
		defer os.Setenv(key, previous)
	}
	server, device := "https://test.invalid", testutil.RemoteAttachDeviceID
	os.Setenv("DESKTOP_TEST_REMOTE_DEVICE", device)
	os.Setenv("DESKTOP_TEST_REMOTE_SERVER", server)
	home := filepath.Join(root, "exact-home")
	second := filepath.Join(root, "second-project")
	if err := os.MkdirAll(second, 0700); err != nil {
		return err
	}
	state := storage.State{Preferences: workspace.Defaults(), Projects: []workspace.Project{{ID: "first", Name: "Scope first", Path: root}, {ID: "second", Name: "Scope second", Path: second}}}
	state.Preferences.SelectedProject = "first"
	state.Sessions = []workspace.Session{{ID: "old", ProjectID: "first", Tool: "fclaude", Name: "Saved remote", Directory: root, Reference: id}, {ID: "neighbor", ProjectID: "first", Tool: "codex", Directory: root}}
	tab := state.Board.Add("first", "old", "Preserved split", "", false)
	state.Board.Add("first", "neighbor", "Preserved split", "old", false)
	tab.Root.Ratio, tab.FocusedSession = 63, "old"
	store, err := storage.Open(home)
	if err != nil {
		return err
	}
	err = store.Save(state)
	store.Close()
	if err != nil {
		return err
	}
	d := driver{root: root}
	var application *app.App
	openApp := func() error {
		mygo.RunOnMain(func() {
			application, err = app.New(home)
			if err == nil {
				d.view = ui.NewTester(application.View, 1366, 768)
			}
		})
		return err
	}
	defer func() {
		if application != nil {
			if err := closeApp(application); result == nil {
				result = err
			}
		}
	}()
	started := 0
	for _, e := range events(root) {
		if e.Kind == "started" {
			started++
		}
	}
	waitText := func(text string) error {
		err := d.wait(func() bool {
			ready := false
			_ = d.action(func(t *ui.Tester) error {
				t.Frame()
				ready = t.HasText(text) && !t.HasText(locales.LoadingRemote())
				return nil
			})
			return ready
		})
		if err == nil {
			return nil
		}
		var texts []string
		_ = d.action(func(t *ui.Tester) error {
			t.Frame()
			texts = t.Texts()
			return nil
		})
		return fmt.Errorf("wait for %q: %w; UI: %v", text, err, texts)
	}
	args := func() []string {
		return []string{"attach", id, "--expected-server", server, "--expected-device", device}
	}
	confirm := func(input string) error {
		if err := d.click(locales.ConfirmRemoteConnection()); err != nil {
			return err
		}
		started++
		if err := d.waitStarted(started); err != nil {
			return err
		}
		if err := d.waitReady(); err != nil {
			return err
		}
		return d.input(input, args())
	}
	// First restore a legacy history through an explicit refreshed selection;
	// then restore the resulting exact history through a new query and consent.
	for _, legacy := range []bool{true, false} {
		if err := openApp(); err != nil {
			return fmt.Errorf("open exact-history app (%t): %w", legacy, err)
		}
		actionLabel := locales.ResumeSession()
		continueLabel := locales.ContinueSession()
		if !legacy {
			// The first exact reconnect replaces the legacy history with a
			// context-bound remote history, which uses remote-specific labels.
			actionLabel = locales.RemoteHistoryReviewAction()
			continueLabel = locales.RemoteHistoryContinue()
		}
		for _, label := range []string{actionLabel, continueLabel} {
			if err := d.click(label); err != nil {
				return fmt.Errorf("click history control %q (%t): %w", label, legacy, err)
			}
		}
		if legacy {
			if err := waitText(locales.RemoteHistorySelectAgain()); err != nil {
				return fmt.Errorf("legacy history selection: %w", err)
			}
			if err := d.action(func(t *ui.Tester) error {
				if t.HasText(locales.ConfirmRemoteConnection()) {
					return fmt.Errorf("legacy history connected without selection")
				}
				return nil
			}); err != nil {
				return err
			}
			if err := d.click(locales.ConnectRemoteSession()); err != nil {
				return fmt.Errorf("legacy history connect control: %w", err)
			}
		}
		if err := waitText(locales.ConfirmRemoteConnection()); err != nil {
			return fmt.Errorf("exact history confirmation (%t): %w", legacy, err)
		}
		if err := d.waitStarted(started); err != nil {
			return fmt.Errorf("exact history preconfirm process count (%t): %w", legacy, err)
		}
		if err := confirm(fmt.Sprintf("history-exact-%t", legacy)); err != nil {
			return fmt.Errorf("exact history confirm (%t): %w", legacy, err)
		}
		if err := closeApp(application); err != nil {
			return fmt.Errorf("close exact-history app (%t): %w", legacy, err)
		}
		application = nil
		got, err := load(home)
		if err != nil {
			return fmt.Errorf("load exact-history state (%t): %w", legacy, err)
		}
		restored := got.Board.Current("first")
		if restored == nil || restored.ID != tab.ID || restored.Root.Ratio != 63 || restored.Root.Leaves()[1] != "neighbor" || restored.Root.Leaves()[0] == "old" || len(got.Sessions) != 2 {
			return fmt.Errorf("remote history lost split placement or duplicated metadata")
		}
		for _, s := range got.Sessions {
			if s.Tool == "fclaude" && (s.Remote == nil || *s.Remote != (workspace.RemoteContext{Home: home, ServerURL: server, DeviceID: device, SessionID: id}) || s.Name != "Saved remote" || s.Directory != root) {
				return fmt.Errorf("remote history lost reviewed public context")
			}
		}
	}
	if err := openApp(); err != nil {
		return fmt.Errorf("open scope app: %w", err)
	}
	openList := func() error {
		if err := d.click(locales.RemoteManagement()); err != nil {
			return err
		}
		if err := waitText(locales.ConnectRemoteSession()); err != nil {
			return err
		}
		return d.click(locales.ConnectRemoteSession())
	}
	for step := range 4 {
		switch step {
		case 1:
			server = "https://other.invalid"
			os.Setenv("DESKTOP_TEST_REMOTE_SERVER", server)
		case 2:
			device = "66666666-6666-4666-8666-666666666666"
			os.Setenv("DESKTOP_TEST_REMOTE_DEVICE", device)
		case 3:
			if err := d.click("Scope second"); err != nil {
				return err
			}
		}
		if err := openList(); err != nil {
			return fmt.Errorf("open scope list step %d: %w", step, err)
		}
		if err := waitText(locales.ConfirmRemoteConnection()); err != nil {
			return fmt.Errorf("scope confirmation step %d: %w", step, err)
		}
		if err := confirm(fmt.Sprintf("distinct-scope-%d", step)); err != nil {
			return fmt.Errorf("scope confirmation step %d: %w", step, err)
		}
	}
	if err := openList(); err != nil {
		return fmt.Errorf("open reused scope list: %w", err)
	}
	if err := d.action(func(t *ui.Tester) error {
		if t.HasText(locales.ConfirmRemoteConnection()) {
			return fmt.Errorf("same exact connection was not reused")
		}
		return nil
	}); err != nil {
		return err
	}
	if err := d.input("same-scope-reused", args()); err != nil {
		return err
	}
	if err := d.waitStarted(started); err != nil {
		return fmt.Errorf("same scope duplicated process: %w", err)
	}
	return nil
}
