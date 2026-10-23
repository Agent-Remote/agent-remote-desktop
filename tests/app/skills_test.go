package app_test

import (
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestSkillsPageShowsProjectedCatalogAndAccountLocalState(t *testing.T) {
	testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1100, 820)
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "https://test.invalid")
	if err := tt.Click(locales.SkillsTitle()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for !tt.HasText("Review helper") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText("Review helper") {
		t.Fatalf("skills page did not show catalog: %v", tt.Texts())
	}
	if !tt.HasText(locales.SkillsGeneration(7)) || !tt.HasText(locales.SkillsLocal()) || !tt.HasText("Account helper") || !tt.HasText(locales.SkillsIncluded()) {
		t.Fatalf("skills projection missing: %v", tt.Texts())
	}
	if tt.HasText("tracking") || tt.HasText("private") {
		t.Fatal("private skill metadata leaked into the desktop view")
	}
	if err := tt.Click(locales.SkillsRefresh()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100 && !tt.HasText("Review helper"); i++ {
		time.Sleep(20 * time.Millisecond)
		tt.Frame()
	}
}

func TestSkillsPageKeepsUnsupportedCapabilityExplicit(t *testing.T) {
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "skills-unsupported")
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1100, 820)
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "https://test.invalid")
	if err := tt.Click(locales.SkillsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SkillsUnsupported())
}

func TestSkillsPageReviewsAndSubmitsEnableDisableChange(t *testing.T) {
	testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1100, 820)
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "https://test.invalid")
	if err := tt.Click(locales.SkillsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "Review helper")
	// The state controls are below the catalog; reveal them in the scroll view.
	tt.Scroll(500, 700, 0, 1200)
	tt.Frame()
	if err := tt.Click(locales.SkillsDisable()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.SkillsChangeReview()) || !tt.HasText(locales.SkillsChangeAction(locales.SkillsDisable())) {
		t.Fatalf("skill change review was not shown: %v", tt.Texts())
	}
	if err := tt.Click(locales.SkillsChangeConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SkillsChangeAccepted(locales.SkillsDisable()))
}

func TestSkillsPageLoadsDedicatedInfoProjection(t *testing.T) {
	testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1100, 820)
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "https://test.invalid")
	if err := tt.Click(locales.SkillsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "Review helper")
	if err := tt.Click(locales.SkillsInfo()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SkillsInfoKind("library"))
	if !tt.HasText(locales.SkillsInfoRevision("1", "revision-1", locales.SkillsInfoRetained())) || tt.HasText("locator") || tt.HasText("tracking") {
		t.Fatalf("dedicated Skill info projection leaked or omitted metadata: %v", tt.Texts())
	}
}

func TestSkillsPageLoadsBoundedStateDiffProjection(t *testing.T) {
	testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1100, 820)
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "https://test.invalid")
	if err := tt.Click(locales.SkillsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "Review helper")
	// State controls follow the catalog; scroll until the checkpoint action is
	// visible, then inspect the bounded path/digest projection.
	tt.Scroll(500, 700, 0, 1600)
	tt.Frame()
	if err := tt.Click(locales.SkillsStateAccountPlaceholder()); err != nil {
		t.Fatal(err)
	}
	tt.Type("11111111-1111-4111-8111-111111111111")
	if err := tt.Click(locales.SkillsStateCurrentDiff()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SkillsStateDiffTitle())
	if !tt.HasText(locales.SkillsStateDiffSummary("current", "1")) {
		t.Fatalf("current state diff projection missing: %v", tt.Texts())
	}
	if err := tt.Click(locales.SkillsStateRefresh()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SkillsStateCheckpoints())
	if err := tt.Click(locales.SkillsStateDiff()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for !tt.HasText(locales.SkillsStateDiffTitle()) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText(locales.SkillsStateDiffTitle()) {
		t.Fatalf("state diff did not load: %v", tt.Texts())
	}
	// The digest includes a dynamic value; README.md and the diff summary are
	// the stable assertions while the redaction check below protects the
	// metadata-only boundary.
	if !tt.HasText("README.md") || !tt.HasText(locales.SkillsStateDiffSummary("checkpoint", "1")) {
		t.Fatalf("state diff projection missing: %v", tt.Texts())
	}
	if tt.HasText("private") {
		t.Fatal("state diff exposed private metadata")
	}
}

func TestSkillsPageOpensExportReviewFromRetainedCheckpoint(t *testing.T) {
	testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1100, 820)
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "https://test.invalid")
	if err := tt.Click(locales.SkillsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "Review helper")
	tt.Scroll(500, 700, 0, 1600)
	tt.Frame()
	if err := tt.Click(locales.SkillsStateAccountPlaceholder()); err != nil {
		t.Fatal(err)
	}
	tt.Type("11111111-1111-4111-8111-111111111111")
	if err := tt.Click(locales.SkillsStateRefresh()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SkillsStateCheckpoints())
	if rect, ok := tt.Find(locales.SkillsStateExport()); ok {
		tt.ClickAt(rect.X+rect.W/2, rect.Y+rect.H/2)
	} else {
		t.Fatal("export checkpoint button is not visible")
	}
	waitRemoteText(t, tt, locales.CliFormTitle())
	if !tt.HasText(locales.SkillsStateExportReview()) {
		t.Fatalf("checkpoint export did not open the reviewed workflow: %v", tt.Texts())
	}
	if !tt.HasText("--output · " + locales.CliFormRequired()) {
		t.Fatalf("checkpoint export review omitted required output destination: %v", tt.Texts())
	}
}
