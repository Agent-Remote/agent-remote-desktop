package theme_test

import (
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/egoist/mygo/ui"
)

func TestSakuraThemeProvidesDecorativeArtworkAndReadableTerminal(t *testing.T) {
	uiTheme := theme.UI("sakura")
	if uiTheme.Background == ui.Transparent || uiTheme.Accent == ui.Transparent {
		t.Fatal("sakura theme did not provide a complete UI palette")
	}
	if theme.Portrait("sakura") == nil {
		t.Fatal("sakura theme did not provide its portrait")
	}
	if theme.Portrait("midnight") != nil {
		t.Fatal("plain terminal theme unexpectedly added decorative art")
	}
	terminalTheme := theme.Terminal("sakura")
	if terminalTheme == nil {
		t.Fatal("sakura terminal theme is unavailable")
	}
	if terminalTheme.Background != uiTheme.Background || terminalTheme.Foreground != uiTheme.Text {
		t.Fatalf("terminal surface does not follow sakura workbench palette: %+v", terminalTheme)
	}
	if terminalTheme.Cursor == ui.Transparent || terminalTheme.CursorText == ui.Transparent || terminalTheme.Selection == ui.Transparent || terminalTheme.SelectionText == ui.Transparent {
		t.Fatalf("terminal interaction colors are incomplete: %+v", terminalTheme)
	}
}

func TestThemeCycleIncludesAllBuiltIns(t *testing.T) {
	if got := theme.Next("midnight"); got != "sakura" {
		t.Fatalf("midnight quick switch = %q, want sakura", got)
	}
	if got := theme.Next("sakura"); got != "daybreak" {
		t.Fatalf("sakura quick switch = %q, want daybreak", got)
	}
	if got := theme.Next("daybreak"); got != "midnight" {
		t.Fatalf("daybreak quick switch = %q, want midnight", got)
	}
	if got := theme.Next("legacy-theme"); got != "midnight" {
		t.Fatalf("unknown theme recovery = %q, want midnight", got)
	}
}

func TestPortraitCollapsesForNarrowSplitAndFocusLayouts(t *testing.T) {
	for _, test := range []struct {
		name        string
		width       float32
		focus       bool
		split       bool
		wantVisible bool
	}{
		{name: "wide workbench", width: theme.PortraitMinWindowWidth, wantVisible: true},
		{name: "narrow window", width: theme.PortraitMinWindowWidth - 1, wantVisible: false},
		{name: "split panes", width: 1366, split: true, wantVisible: false},
		{name: "focus mode", width: 1366, focus: true, wantVisible: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := theme.PortraitVisible(test.width, test.focus, test.split); got != test.wantVisible {
				t.Fatalf("portrait visibility = %v, want %v", got, test.wantVisible)
			}
		})
	}
}

func TestEveryBuiltInTerminalKeepsInteractionColorsReadable(t *testing.T) {
	for _, name := range theme.ThemeNames {
		t.Run(name, func(t *testing.T) {
			uiTheme := theme.UI(name)
			terminalTheme := theme.Terminal(name)
			if terminalTheme.Background != uiTheme.Background || terminalTheme.Foreground != uiTheme.Text {
				t.Fatalf("terminal base colors diverge from workbench: %+v", terminalTheme)
			}
			if terminalTheme.Cursor == ui.Transparent || terminalTheme.CursorText == ui.Transparent || terminalTheme.Selection == ui.Transparent || terminalTheme.SelectionText == ui.Transparent {
				t.Fatalf("terminal interaction colors are not explicit: %+v", terminalTheme)
			}
		})
	}
}
