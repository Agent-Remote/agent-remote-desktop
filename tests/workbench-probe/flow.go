package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type driver struct {
	view *ui.Tester
	root string
}

func (d *driver) action(fn func(*ui.Tester) error) error {
	var err error
	mygo.RunOnMain(func() { err = fn(d.view) })
	return err
}
func (d *driver) click(label string) error {
	return d.action(func(t *ui.Tester) error { return t.Click(label) })
}
func (d *driver) fill(label, value string) error {
	return d.action(func(t *ui.Tester) error {
		if err := t.Click(label); err != nil {
			return err
		}
		t.Type(value)
		return nil
	})
}
func (d *driver) key(mods ui.Modifiers, key ui.Key) error {
	return d.action(func(t *ui.Tester) error { t.Key(mods, key); return nil })
}
func (d *driver) settle() error {
	return d.action(func(t *ui.Tester) error {
		t.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
		t.Frame()
		t.Advance(20 * time.Millisecond)
		return nil
	})
}
func (d *driver) wait(fn func() bool) error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return fmt.Errorf("workbench probe timed out; events: %+v", events(d.root))
}
func (d *driver) waitStarted(n int) error {
	err := d.wait(func() bool {
		count := 0
		for _, e := range events(d.root) {
			if e.Kind == "started" {
				count++
			}
		}
		return count == n
	})
	if err != nil {
		count := 0
		for _, e := range events(d.root) {
			if e.Kind == "started" {
				count++
			}
		}
		return fmt.Errorf("wait for %d started processes (observed %d): %w", n, count, err)
	}
	return nil
}
func (d *driver) waitReady() error {
	// Launch completion is already synchronized by waitStarted. Render a
	// settled frame so the newly focused terminal owns keyboard input; a global
	// "Starting" label is not a reliable readiness signal with split panes.
	return d.settle()
}
func (d *driver) launch(name string, split bool, n int) error {
	// The redesign removed the old toolbar buttons. Open the current create
	// wheel through its shortcut and choose Advanced for a regular session;
	// the split shortcut opens the same dialog with the active pane as target.
	if split {
		if err := d.key(ui.Cmd|ui.Shift, ui.KeyD); err != nil {
			return err
		}
		if err := d.settle(); err != nil {
			return err
		}
	} else {
		if err := d.key(ui.Cmd, ui.KeyT); err != nil {
			return err
		}
		if err := d.settle(); err != nil {
			return err
		}
		if err := d.click(locales.CreateAdvanced()); err != nil {
			return err
		}
		if err := d.settle(); err != nil {
			return err
		}
	}
	if err := d.fill(locales.SessionName(), name); err != nil {
		return err
	}
	if err := d.fill(locales.LaunchArgs(), fmt.Sprintf(`["%s"]`, name)); err != nil {
		return err
	}
	if err := d.click(locales.StartSession()); err != nil {
		return err
	}
	if err := d.waitStarted(n); err != nil {
		return err
	}
	return d.waitReady()
}
func (d *driver) input(value string, args []string) error {
	if err := d.action(func(t *ui.Tester) error { t.Type(value); t.Key(0, ui.KeyEnter); return nil }); err != nil {
		return err
	}
	return d.wait(func() bool {
		for _, e := range events(d.root) {
			if e.Kind == "input" && e.Input == value && reflect.DeepEqual(e.Args, args) {
				return true
			}
		}
		return false
	})
}
func load(home string) (storage.State, error) {
	s, err := storage.Open(home)
	if err != nil {
		return storage.State{}, err
	}
	defer s.Close()
	return s.Load()
}

func runProbe(root string) (result error) {
	home := filepath.Join(root, "home")
	project, err := workspace.NewProject(root)
	if err != nil {
		return err
	}
	state := storage.State{Projects: []workspace.Project{project}, Preferences: workspace.Defaults()}
	state.Preferences.SelectedProject = project.ID
	store, err := storage.Open(home)
	if err != nil {
		return err
	}
	err = store.Save(state)
	store.Close()
	if err != nil {
		return err
	}
	var application *app.App
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
		if application != nil {
			if err := closeApp(application); result == nil {
				result = err
			}
		}
	}()
	if err = d.launch("Alpha", false, 1); err != nil {
		return err
	}
	if err = d.launch("Beta", false, 2); err != nil {
		return err
	}
	if err = d.launch("Gamma", true, 3); err != nil {
		return err
	}
	if err = d.input("gamma-in-split", []string{"Gamma"}); err != nil {
		return err
	}
	_ = d.key(ui.Ctrl, ui.KeyTab)
	if err = d.input("alpha-after-tab", []string{"Alpha"}); err != nil {
		return err
	}
	_ = d.key(ui.Ctrl, ui.KeyTab)
	if err = d.input("gamma-restored-focus", []string{"Gamma"}); err != nil {
		return err
	}
	_ = d.key(0, ui.KeyF6)
	if err = d.input("beta-next-pane", []string{"Beta"}); err != nil {
		return err
	}
	// The redesign keeps tab navigation in the Agents dock (⌥A). Open it and
	// dismiss it again; dedicated app tests cover card reordering and hiding,
	// while this native probe keeps the persisted split lifecycle deterministic.
	if err = d.key(ui.Alt, ui.KeyA); err != nil {
		return err
	}
	if err = d.settle(); err != nil {
		return err
	}
	if err = d.key(0, ui.KeyEscape); err != nil {
		return err
	}
	if err = d.settle(); err != nil {
		return err
	}
	if err = closeApp(application); err != nil {
		return err
	}
	application = nil
	before, err := load(home)
	if err != nil {
		return err
	}
	tab := before.Board.Current(project.ID)
	if tab == nil || len(tab.Root.Leaves()) != 2 || len(before.Sessions) != 3 {
		var leaves []string
		if tab != nil {
			leaves = tab.Root.Leaves()
		}
		return fmt.Errorf("workspace snapshot missing panes or sessions: tab=%t leaves=%v sessions=%d tabs=%d", tab != nil, leaves, len(before.Sessions), len(before.Board.Tabs))
	}
	oldIDs := tab.Root.Leaves()
	// No launch may happen merely because the saved layout is opened.
	mygo.RunOnMain(func() {
		application, err = app.New(home)
		if err == nil {
			d.view = ui.NewTester(application.View, 1366, 768)
		}
	})
	if err != nil {
		return err
	}
	if err = d.action(func(t *ui.Tester) error {
		// Restored panes surface the current redesign's "Saved session" heading;
		// the "Not connected" status is only rendered inside the Agents dock.
		saved, running := t.HasText(locales.SavedSession()), t.HasText(locales.Running())
		if !saved || running {
			return fmt.Errorf("history displayed as running; saved=%v running=%v texts: %q", saved, running, t.Texts())
		}
		return nil
	}); err != nil {
		return err
	}
	if err = d.waitStarted(3); err != nil {
		return err
	}
	if err = d.click(locales.ResumeSession()); err != nil {
		return err
	}
	if err = d.fill(locales.LocalResumeReference(), "probe-history-id"); err != nil {
		return err
	}
	if err = d.click(locales.ContinueSession()); err != nil {
		return err
	}
	if err = d.waitStarted(4); err != nil {
		return err
	}
	if err = d.waitReady(); err != nil {
		return err
	}
	if err = d.input("after-explicit-resume", []string{"resume", "probe-history-id"}); err != nil {
		return err
	}
	if err = closeApp(application); err != nil {
		return err
	}
	application = nil
	after, err := load(home)
	if err != nil {
		return err
	}
	restored := after.Board.Current(project.ID)
	if restored == nil || restored.ID != tab.ID || restored.Root.Ratio != tab.Root.Ratio || restored.Root.Leaves()[0] == oldIDs[0] || restored.Root.Leaves()[1] != oldIDs[1] {
		return fmt.Errorf("explicit resume lost pane location or neighbor")
	}
	if len(after.Sessions) != 3 {
		return fmt.Errorf("resume duplicated history")
	}
	for _, saved := range after.Sessions {
		if strings.Contains(saved.Reference, "Alpha") {
			return fmt.Errorf("old launch arguments were saved as recovery identity")
		}
	}
	return nil
}
