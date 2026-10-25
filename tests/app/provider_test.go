package app_test

import (
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

type providerFixture struct {
	available map[string]bool
	calls     int
	directory string
}

func (f *providerFixture) resolve(name string) (string, error) {
	f.calls++
	if !f.available[name] {
		return "", fault.ExecutableMissing
	}
	return filepath.Join(f.directory, name), nil
}

func providerTester(t *testing.T, locale string, tool session.Tool, restored bool) (*ui.Tester, *providerFixture) {
	t.Helper()
	i18n.SetLocaleFunc(func() string { return locale })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	state := testutil.SavedWorkspace(t)
	state.Sessions = nil
	state.Board = workspace.Board{}
	if restored {
		state.Sessions = []workspace.Session{{ID: "provider", ProjectID: "one", Tool: string(tool), Directory: state.Projects[0].Path}}
		state.Board.Add("one", "provider", "Provider", "", false)
	}
	testutil.StoreWorkspace(t, home, state)
	resolver := &providerFixture{available: map[string]bool{}, directory: t.TempDir()}
	a, err := app.NewWithOptions(home, app.Options{ChordTapWindow: 10 * time.Second, FindExecutable: resolver.resolve})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Error(err)
		}
	})
	tt := ui.NewTester(a.View, 900, 600)
	tt.SetLocale(locale)
	return tt, resolver
}

func providerLabel(tool session.Tool) string {
	if tool == session.RemoteClaude {
		return locales.RemoteTool(string(tool))
	}
	return locales.LocalTool(string(tool))
}

func TestMissingProvidersDisableNewSessionAndRefreshDetectionWithoutExecutingTools(t *testing.T) {
	for _, tool := range []session.Tool{session.Codex, session.Claude, session.RemoteClaude} {
		t.Run(string(tool), func(t *testing.T) {
			tt, resolver := providerTester(t, "en", tool, false)
			calls := resolver.calls
			for range 5 {
				tt.Frame()
			}
			if resolver.calls != calls {
				t.Fatal("redrawing repeated executable discovery")
			}
			if err := openNewSessionDialog(tt); err != nil {
				t.Fatal(err)
			}
			calls = resolver.calls
			for range 5 {
				tt.Frame()
			}
			if resolver.calls != calls {
				t.Fatal("dialog redraw repeated executable discovery")
			}
			if err := tt.Click(providerLabel(tool)); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.ProviderRequired(string(tool))) {
				t.Fatal("missing provider has no explanation")
			}
			if err := tt.Click(locales.StartSession()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.NewAgentSession()) || tt.HasText(locales.ErrorExecutableMissing()) {
				t.Fatal("disabled startup still ran")
			}
			if len(tt.OpenedURLs()) != 0 {
				t.Fatal("opened installation guide automatically")
			}
			if err := tt.Click(locales.ProviderInstallGuide(string(tool))); err != nil {
				t.Fatal(err)
			}
			expectedGuide := map[session.Tool]string{session.Codex: "https://github.com/openai/codex", session.Claude: "https://code.claude.com/docs/en/setup", session.RemoteClaude: "https://github.com/Agent-Remote/agent-remote-cli#release-packaging"}[tool]
			if urls := tt.OpenedURLs(); len(urls) != 1 || urls[0] != expectedGuide {
				t.Fatalf("wrong installation guide: %v", urls)
			}
			resolver.available[string(tool)] = true
			if err := tt.Click(locales.RefreshTools()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.ProviderReady(string(tool))) {
				t.Fatal("refresh failed to detect newly installed provider")
			}
			// Availability can change after the displayed snapshot. Launch must check
			// again and keep the form when the executable disappeared.
			resolver.available[string(tool)] = false
			if err := tt.Click(locales.StartSession()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.NewAgentSession()) || !tt.HasText(locales.ProviderRequired(string(tool))) || !tt.HasText(locales.ErrorExecutableMissing()) {
				t.Fatal("launch trusted stale availability")
			}
		})
	}
}

func TestMissingProvidersDisableRecoveryAndKeepExplicitRemoteTarget(t *testing.T) {
	for _, tool := range []session.Tool{session.Codex, session.Claude, session.RemoteClaude} {
		t.Run(string(tool), func(t *testing.T) {
			tt, resolver := providerTester(t, "en", tool, true)
			location := locales.Local()
			if tool == session.RemoteClaude {
				location = locales.Remote()
			}
			// The restored session is the project's only pane: its card in the
			// ⌥A dock replaces the removed sidebar row.
			if err := openDockCard(tt, string(tool)); err != nil {
				t.Fatal(err)
			}
			_ = location
			if err := tt.Click(locales.ResumeSession()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.ProviderRequired(string(tool))) {
				t.Fatal("missing provider recovery has no explanation")
			}
			if tool == session.RemoteClaude {
				if !tt.HasText(locales.RemoteHistoryReviewHelp()) {
					t.Fatal("lost attach semantics")
				}

			} else if !tt.HasText(locales.LocalResumeHelp()) {
				t.Fatal("lost native history picker semantics")
			}
			if err := tt.Click(locales.ContinueSession()); err != nil {
				t.Fatal(err)
			}
			if tt.HasText(locales.ErrorExecutableMissing()) {
				t.Fatal("disabled recovery ran")
			}
			resolver.available[string(tool)] = true
			if err := tt.Click(locales.RefreshTools()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.ProviderReady(string(tool))) {
				t.Fatal("refresh did not enable recovery")
			}
			resolver.available[string(tool)] = false
			if err := tt.Click(locales.ContinueSession()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.ErrorExecutableMissing()) || !tt.HasText(locales.ContinueSession()) {
				t.Fatal("recovery trusted stale provider detection")
			}
		})
	}
}

func TestProviderGuideFailureAndMinimumLocalizedDialogs(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			tt, _ := providerTester(t, language, session.Codex, false)
			if err := openNewSessionDialog(tt); err != nil {
				t.Fatal(err)
			}
			for _, label := range []string{locales.NewAgentSession(), locales.StartSession(), locales.Cancel(), locales.ProviderRequired("codex"), locales.ProviderInstallGuide("codex"), locales.RefreshTools()} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 901 || r.Y+r.H > 601 {
					t.Errorf("%q outside window: %+v", label, r)
				}
			}
			if output := os.Getenv("DESKTOP_RENDER_DIR"); output != "" {
				if err := os.MkdirAll(output, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(output, "provider-new-"+language+".png"))
				if err != nil {
					t.Fatal(err)
				}
				if err = png.Encode(f, tt.Image()); err != nil {
					f.Close()
					t.Fatal(err)
				}
				f.Close()
			}
			tt.FailOpenURL(errors.New("browser unavailable"))
			if err := tt.Click(locales.ProviderInstallGuide("codex")); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.ProviderGuideOpenFailed()) {
				t.Fatal("guide failure lacked localized recovery")
			}
			if err := tt.Click(locales.Cancel()); err != nil {
				t.Fatal(err)
			}
			tt, _ = providerTester(t, language, session.Codex, true)
			if err := tt.Click(locales.ResumeSession()); err != nil {
				t.Fatal(err)
			}
			for _, label := range []string{locales.ResumeSession(), locales.ContinueSession(), locales.Cancel(), locales.ProviderInstallGuide("codex"), locales.RefreshTools()} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 901 || r.Y+r.H > 601 {
					t.Errorf("recovery %q outside window: %+v", label, r)
				}
			}
			if output := os.Getenv("DESKTOP_RENDER_DIR"); output != "" {
				f, err := os.Create(filepath.Join(output, "provider-recovery-"+language+".png"))
				if err != nil {
					t.Fatal(err)
				}
				if err = png.Encode(f, tt.Image()); err != nil {
					f.Close()
					t.Fatal(err)
				}
				f.Close()
			}
		})
	}
}
