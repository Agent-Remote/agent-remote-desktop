package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func wireguardTester(t *testing.T, mode, language string) (*ui.Tester, *app.App, string, string) {
	t.Helper()
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
	i18n.SetLocaleFunc(func() string { return language })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home, log := t.TempDir(), filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	a, err := app.New(home)
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
	for _, label := range []string{locales.DiagnosticsTitle(), locales.WireGuardConfigTitle()} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	return tt, a, home, log
}

func reviewWireGuard(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click(locales.ReviewWireGuardConfig()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.WireGuardReviewHelp())
}

func TestWireGuardReviewAndExactConfirmation(t *testing.T) {
	for _, custom := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "custom"}[custom], func(t *testing.T) {
			mode := ""
			if custom {
				mode = "wg-replace"
			}
			tt, _, home, log := wireguardTester(t, mode, "en")
			output := filepath.Join(home, "wireguard", "agent-remote.conf")
			if custom {
				output = filepath.Join(t.TempDir(), "my network.conf")
				for _, label := range []string{locales.WireGuardCustomOutput(), locales.WireGuardOutputPath()} {
					if err := tt.Click(label); err != nil {
						t.Fatal(err)
					}
				}
				tt.Type(output)
			}
			planCapture, applyCapture := filepath.Join(t.TempDir(), "plan"), filepath.Join(t.TempDir(), "apply")
			t.Setenv("DESKTOP_TEST_WIREGUARD_PLAN", planCapture)
			t.Setenv("DESKTOP_TEST_WIREGUARD_APPLY", applyCapture)
			reviewWireGuard(t, tt)
			action := "create"
			if custom {
				action = "replace"
			}
			for _, text := range []string{home, "https://test.invalid", "device-1", output, locales.WireGuardOutputAction(action)} {
				if !tt.HasText(text) {
					t.Errorf("confirmation omitted %q", text)
				}
			}
			calls, _ := os.ReadFile(log)
			if strings.Contains(string(calls), "wireguard.config.apply") {
				t.Fatal("review submitted mutation")
			}
			if _, err := os.Stat(filepath.Join(home, "wireguard")); !os.IsNotExist(err) {
				t.Fatal("review created CLI data")
			}
			data, err := os.ReadFile(planCapture)
			if err != nil {
				t.Fatal(err)
			}
			var request struct {
				Home   string  `json:"expected_home"`
				Output *string `json:"output"`
			}
			if err := json.Unmarshal(data, &request); err != nil {
				t.Fatal(err)
			}
			if request.Home != home || (custom && (request.Output == nil || *request.Output != output)) || (!custom && request.Output != nil) {
				t.Fatalf("unexpected output selection: %s", data)
			}
			if err := tt.Click(locales.ConfirmWireGuardConfig()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.WireGuardConfigComplete())
			data, err = os.ReadFile(applyCapture)
			if err != nil {
				t.Fatal(err)
			}
			var confirmed struct {
				Confirmed bool `json:"confirmed"`
				Plan      struct {
					Version int               `json:"plan_version"`
					Home    string            `json:"home"`
					Context map[string]string `json:"context"`
					Output  struct {
						Path           string   `json:"path"`
						Action         string   `json:"action"`
						AnchorPath     string   `json:"anchor_path"`
						AnchorIdentity string   `json:"anchor_identity"`
						Missing        []string `json:"missing_directories"`
						Target         *string  `json:"target_identity"`
					} `json:"output"`
				} `json:"expected_plan"`
			}
			if err := json.Unmarshal(data, &confirmed); err != nil {
				t.Fatal(err)
			}
			p := confirmed.Plan
			if !confirmed.Confirmed || p.Version != 1 || p.Home != home || p.Context["server_url"] != "https://test.invalid" || p.Context["active_device_id"] != "device-1" || p.Output.Path != output || p.Output.Action != action || p.Output.AnchorIdentity != strings.Repeat("a", 64) || p.Output.Missing == nil {
				t.Fatalf("lost exact plan identity: %s", data)
			}
			if custom && (p.Output.Target == nil || *p.Output.Target != strings.Repeat("b", 64)) {
				t.Fatal("overwrite confirmation lost target identity")
			}
			calls, _ = os.ReadFile(log)
			if strings.Count(string(calls), "wireguard.config.apply\n") != 1 {
				t.Fatalf("mutation replay: %q", calls)
			}
		})
	}
}

func TestWireGuardEditingOutputRequiresANewPlan(t *testing.T) {
	tt, _, _, log := wireguardTester(t, "", "en")
	reviewWireGuard(t, tt)
	if err := tt.Click(locales.EditWireGuardConfig()); err != nil {
		t.Fatal(err)
	}
	if tt.HasText(locales.ConfirmWireGuardConfig()) {
		t.Fatal("editing retained confirmation")
	}
	for _, label := range []string{locales.WireGuardCustomOutput(), locales.WireGuardOutputPath()} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	tt.Type("relative.conf")
	if err := tt.Click(locales.ReviewWireGuardConfig()); err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "wireguard.config.plan\n") != 1 {
		t.Fatal("relative output sent for confirmation")
	}
	// Returning to default discards the custom field from the request and creates
	// a new plan instead of silently reusing the earlier confirmation.
	if err := tt.Click(locales.WireGuardDefaultOutput()); err != nil {
		t.Fatal(err)
	}
	reviewWireGuard(t, tt)
	if err := tt.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	calls, _ = os.ReadFile(log)
	if strings.Count(string(calls), "wireguard.config.plan\n") != 2 || strings.Contains(string(calls), "wireguard.config.apply") {
		t.Fatalf("unexpected plan or mutation calls: %q", calls)
	}
}

func TestWireGuardUnsupportedDoesNotOfferConfirmation(t *testing.T) {
	for _, mode := range []string{"wg-unsupported", "wg-readonly"} {
		t.Run(mode, func(t *testing.T) {
			tt, _, _, log := wireguardTester(t, mode, "en")
			if err := tt.Click(locales.ReviewWireGuardConfig()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.WireGuardUnsupported())
			if tt.HasText(locales.ConfirmWireGuardConfig()) {
				t.Fatal("unsupported method offered confirmation")
			}
			calls, _ := os.ReadFile(log)
			if strings.Contains(string(calls), "wireguard.config.") {
				t.Fatalf("unsupported CLI received a plan or write: %q", calls)
			}
		})
	}
}

func TestWireGuardPartialAndUnknownReceiptsDoNotReplay(t *testing.T) {
	for _, mode := range []string{"wg-partial", "wg-enrollment-unknown", "wg-key-store-incomplete", "wg-publication-incomplete", "wg-apply-unknown", "wg-malformed", "wg-wrong-operation", "wg-apply-output-changed"} {
		t.Run(mode, func(t *testing.T) {
			tt, _, _, log := wireguardTester(t, mode, "en")
			reviewWireGuard(t, tt)
			if err := tt.Click(locales.ConfirmWireGuardConfig()); err != nil {
				t.Fatal(err)
			}
			want := locales.WireGuardConfigIncomplete()
			if mode == "wg-apply-unknown" || mode == "wg-malformed" || mode == "wg-wrong-operation" {
				want = locales.WireGuardOutcomeUnknown()
			}
			if mode == "wg-apply-output-changed" {
				want = locales.WireGuardResultCode("output_changed")
			}
			waitRemoteText(t, tt, want)
			if tt.HasText(locales.WireGuardConfigComplete()) || tt.HasText(locales.ConfirmWireGuardConfig()) || tt.HasText(locales.ReviewWireGuardConfig()) {
				t.Fatal("uncertain result reported completion or offered blind retry")
			}
			for _, label := range []string{locales.Settings(), locales.WireGuardOperationStatus()} {
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			if !tt.HasText(want) {
				t.Fatal("navigation discarded receipt")
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "wireguard.config.apply\n") != 1 {
				t.Fatalf("mutation repeated: %q", calls)
			}
		})
	}
}

func TestWireGuardMinimumLocalizedReviewAndResult(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			tt, _, _, _ := wireguardTester(t, "wg-replace", language)
			reviewWireGuard(t, tt)
			assertWireGuardControls(t, tt, locales.WireGuardConfigTitle(), locales.EditWireGuardConfig(), locales.ConfirmWireGuardConfig(), locales.Cancel())
			renderLocalSetup(t, tt, "wireguard-review-"+language)
			if err := tt.Click(locales.ConfirmWireGuardConfig()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.WireGuardConfigComplete())
			assertWireGuardControls(t, tt, locales.WireGuardConfigTitle(), locales.ReturnDiagnostics(), locales.WireGuardConfigComplete(), locales.WireGuardKeyState("reused"), locales.WireGuardEnrollment("confirmed"), locales.WireGuardPublication("published"), locales.WireGuardPeerCount(2))
			renderLocalSetup(t, tt, "wireguard-result-"+language)
		})
	}
}

func assertWireGuardControls(t *testing.T, tt *ui.Tester, labels ...string) {
	t.Helper()
	for _, label := range labels {
		rect, ok := tt.Find(label)
		if !ok || rect.X < 0 || rect.Y < 0 || rect.X+rect.W > 901 || rect.Y+rect.H > 601 {
			t.Errorf("control %q outside minimum window: %+v", label, rect)
		}
	}
}
