package app_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

const attachSessionID = "11111111-1111-4111-8111-111111111111"

func attachHistoryTester(t *testing.T, language, mode string, edit func(*storage.State, string)) (*app.App, *ui.Tester, string, string, storage.State) {
	t.Helper()
	binary := testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_REMOTE_DEVICE", testutil.RemoteAttachDeviceID)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
	i18n.SetLocaleFunc(func() string { return language })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	state := testutil.SavedWorkspace(t)
	state.Sessions[2].Reference = attachSessionID
	state.Sessions[2].Remote = &workspace.RemoteContext{Home: home, ServerURL: "https://test.invalid", DeviceID: testutil.RemoteAttachDeviceID, SessionID: attachSessionID}
	if edit != nil {
		edit(&state, home)
	}
	testutil.StoreWorkspace(t, home, state)
	a, err := app.NewWithOptions(home, app.Options{FindExecutable: func(name string) (string, error) {
		if name == "agent-remote" {
			return binary, nil
		}
		if name == "fclaude" {
			return filepath.Join(home, "fclaude"), nil
		}
		return "", fault.ExecutableMissing
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Error(err)
		}
	})
	tt := ui.NewTester(a.View, 900, 600)
	tt.SetLocale(language)
	return a, tt, home, log, state
}

func reviewHistory(t *testing.T, tt *ui.Tester) {
	t.Helper()
	// Select the persisted remote tab first. This gives the saved remote pane a
	// stable action target even when pseudo-localized labels share text with
	// other saved panes in the sidebar.
	if err := tt.Click("Remote"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	action, continuation := locales.RemoteHistoryReviewAction(), locales.RemoteHistoryContinue()
	if !tt.HasText(action) {
		// Legacy history has no saved remote context and keeps the generic
		// recovery action until the user selects a refreshed target.
		action, continuation = locales.ResumeSession(), locales.ContinueSession()
	}
	for _, label := range []string{action, continuation} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
	}
}

func TestRemoteHistoryRefreshesExactIdentityBeforeConfirming(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			a, tt, home, log, state := attachHistoryTester(t, language, "", nil)
			reviewHistory(t, tt)
			waitRemoteText(t, tt, locales.ConfirmRemoteConnection())
			for _, text := range []string{attachSessionID, "https://test.invalid", locales.RemoteAttachDevice(testutil.RemoteAttachDeviceID), home, state.Sessions[2].Directory, locales.RemoteAttachConfirmation()} {
				if !tt.HasText(text) {
					t.Errorf("missing reviewed identity %q", text)
				}
			}
			for _, label := range []string{locales.Cancel(), locales.ConfirmRemoteConnection()} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 901 || r.Y+r.H > 601 {
					t.Errorf("footer outside window: %q %+v", label, r)
				}
			}
			captureSyncSnapshot(t, tt, language+"-remote-exact-history")
			calls, err := os.ReadFile(log)
			if err != nil || strings.Count(string(calls), "sessions.list\n") != 1 {
				t.Fatalf("history did not refresh exact list: %s %v", calls, err)
			}
			if err := tt.Click(locales.Cancel()); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click(locales.RefreshRemote()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, attachSessionID)
			if tt.HasText(locales.ConfirmRemoteConnection()) {
				t.Fatal("canceled history silently reopened confirmation")
			}
			if err := a.Close(); err != nil {
				t.Fatal(err)
			}
			if got := testutil.LoadWorkspace(t, home); !reflect.DeepEqual(got.Sessions, state.Sessions) {
				t.Fatal("canceled review altered history")
			}
		})
	}
}

func TestLegacyRemoteHistoryRequiresExplicitRefreshedSelection(t *testing.T) {
	a, tt, home, log, state := attachHistoryTester(t, "en", "", func(s *storage.State, _ string) { s.Sessions[2].Remote = nil })
	reviewHistory(t, tt)
	waitRemoteText(t, tt, locales.RemoteHistorySelectAgain())
	if tt.HasText(locales.ConfirmRemoteConnection()) {
		t.Fatal("legacy UUID silently bound to current server")
	}
	// The history review content is in the scrolling list at minimum size.
	scrollRemoteTo(t, tt, locales.ConnectRemoteSession())
	if err := tt.Click(locales.ConnectRemoteSession()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.ConfirmRemoteConnection()) || !tt.HasText(state.Sessions[2].Directory) || !tt.HasText(locales.RemoteAttachLocalProject(state.Sessions[2].ProjectID)) {
		t.Fatal("explicit selection lost historical project context")
	}
	if err := tt.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "attach") || strings.Contains(string(calls), "create") {
		t.Fatalf("review mutated remote session: %s", calls)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if got := testutil.LoadWorkspace(t, home); !reflect.DeepEqual(got.Sessions, state.Sessions) {
		t.Fatal("selection/cancel replaced legacy history")
	}
}

func TestRemoteHistoryRejectsChangedContextAndUnavailableTargets(t *testing.T) {
	for _, tc := range []struct {
		name, mode string
		edit       func(*storage.State, string)
		notice     func() string
	}{
		{"home", "", func(s *storage.State, h string) { s.Sessions[2].Remote.Home = filepath.Join(h, "other") }, locales.RemoteHistoryContextChanged},
		{"server", "", func(s *storage.State, _ string) { s.Sessions[2].Remote.ServerURL = "https://other.invalid" }, locales.RemoteHistoryContextChanged},
		{"device", "", func(s *storage.State, _ string) {
			s.Sessions[2].Remote.DeviceID = "66666666-6666-4666-8666-666666666666"
		}, locales.RemoteHistoryContextChanged},
		{"missing", "remote-attach-missing", nil, locales.RemoteHistoryUnavailable},
		{"stopped", "remote-attach-stopped", nil, locales.RemoteHistoryUnavailable},
		{"unsupported", "remote-attach-unsupported", nil, locales.RemoteAttachUnsupported},
		{"future", "remote-attach-future", nil, locales.RemoteAttachUnsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, tt, home, _, state := attachHistoryTester(t, "en", tc.mode, tc.edit)
			reviewHistory(t, tt)
			waitRemoteText(t, tt, tc.notice())
			if tt.HasText(locales.ConfirmRemoteConnection()) {
				t.Fatal("unverified history was connectable")
			}
			if tc.name == "server" || tc.name == "device" || tc.name == "home" {
				scrollRemoteTo(t, tt, locales.ConnectRemoteSession())
				if err := tt.Click(locales.ConnectRemoteSession()); err != nil {
					t.Fatal(err)
				}
				if tt.HasText(locales.ConfirmRemoteConnection()) {
					t.Fatal("list click reassigned exact history to another context")
				}
			}
			if err := a.Close(); err != nil {
				t.Fatal(err)
			}
			if got := testutil.LoadWorkspace(t, home); !reflect.DeepEqual(got.Sessions, state.Sessions) {
				t.Fatal("failed refresh altered saved history")
			}
		})
	}
}

func TestRemoteListRejectsUnsupportedAndAmbiguousAttachments(t *testing.T) {
	for _, mode := range []string{"remote-attach-unsupported", "remote-attach-future", "remote-attach-prefix", "remote-attach-duplicate"} {
		t.Run(mode, func(t *testing.T) {
			_, tt, _, _, _ := attachHistoryTester(t, "en", mode, nil)
			if err := tt.Click(locales.RemoteManagement()); err != nil {
				t.Fatal(err)
			}
			notice := locales.RemoteAttachContextInvalid()
			if mode == "remote-attach-unsupported" || mode == "remote-attach-future" {
				notice = locales.RemoteAttachUnsupported()
			}
			if mode == "remote-attach-duplicate" {
				notice = presentation.Error(fault.CLIProtocol)
				waitRemoteText(t, tt, notice)
				if tt.HasText(locales.ConnectRemoteSession()) {
					t.Fatal("duplicate targets remained connectable")
				}
				return
			}
			waitRemoteText(t, tt, notice)
			scrollRemoteTo(t, tt, locales.ConnectRemoteSession())
			if err := tt.Click(locales.ConnectRemoteSession()); err != nil {
				t.Fatal(err)
			}
			if tt.HasText(locales.ConfirmRemoteConnection()) {
				t.Fatal("unsupported/ambiguous target reached confirmation")
			}
		})
	}
}

func TestRemoteActiveSessionRemainsAttachable(t *testing.T) {
	_, tt, _, _, _ := attachHistoryTester(t, "en", "remote-attach-active", nil)
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, attachSessionID)
	scrollRemoteTo(t, tt, locales.ConnectRemoteSession())
	if err := tt.Click(locales.ConnectRemoteSession()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.ConfirmRemoteConnection()) {
		t.Fatal("active Server session was incorrectly treated as unavailable")
	}
}

func TestRemoteRefreshFailureAndCancellationDiscardOldAttachment(t *testing.T) {
	for _, mode := range []string{"wrong-id", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			a, tt, home, _, state := attachHistoryTester(t, "en", "", nil)
			if err := tt.Click(locales.RemoteManagement()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, attachSessionID)
			if err := tt.Click(locales.ConnectRemoteSession()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.ConfirmRemoteConnection()) {
				t.Fatal("fixture never reached confirmation")
			}
			if err := tt.Click(locales.Cancel()); err != nil {
				t.Fatal(err)
			}
			expected := locales.ErrorCLIProtocol()
			if mode == "cancel" {
				t.Setenv("DESKTOP_TEST_REMOTE_RELEASE", filepath.Join(t.TempDir(), "not-released"))
				expected = locales.RemoteQueryCanceled()
			} else {
				t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			}
			if err := tt.Click(locales.RefreshRemote()); err != nil {
				t.Fatal(err)
			}
			if tt.HasText(locales.ConnectRemoteSession()) || tt.HasText(locales.ConfirmRemoteConnection()) {
				t.Fatal("in-flight refresh exposed stale attachment")
			}
			if mode == "cancel" {
				if err := tt.Click(locales.CancelRemoteQuery()); err != nil {
					t.Fatal(err)
				}
			}
			waitRemoteText(t, tt, expected)
			if tt.HasText(locales.ConnectRemoteSession()) || tt.HasText(locales.ConfirmRemoteConnection()) {
				t.Fatal("failed refresh exposed stale attachment")
			}
			if err := a.Close(); err != nil {
				t.Fatal(err)
			}
			if got := testutil.LoadWorkspace(t, home); !reflect.DeepEqual(got.Sessions, state.Sessions) {
				t.Fatal("failed query altered saved history")
			}
		})
	}
}

func scrollRemoteTo(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	for attempt := 0; attempt < 30; attempt++ {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("missing remote target %q", label)
		}
		top := float32(390)
		if filter, found := tt.Find(locales.FilterRemote()); found {
			top = filter.Y + filter.H + 12
		}
		if r.Y >= top && r.Y+r.H <= 530 && r.H > 0 {
			return
		}
		delta := r.Y + r.H - 520
		if r.Y < top {
			delta = r.Y - top
		}
		tt.Scroll(650, 480, 0, delta)
	}
	captureSyncSnapshot(t, tt, "scroll-failure")
	r, _ := tt.Find(label)
	t.Fatalf("cannot scroll to remote target %q: %+v", label, r)
}
