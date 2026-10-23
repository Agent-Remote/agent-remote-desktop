// live-pages-probe opens every Control Center destination of the real
// desktop (headless, ui.Tester) against a real agent-remote CLI and the
// server its home is signed in to, and reports what each page showed. It
// only reads: no page is submitted, nothing is confirmed. Run it against a
// disposable home and server (redesign §10.6, R5):
//
//	go run ./tests/live-pages-probe -home /tmp/cli-home -cli /path/to/agent-remote
//
// A page fails when it shows a protocol/contract error, or when its own
// read never settles within the timeout.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

type page struct {
	Tile string `json:"tile"`
	// Gated is a tile the CLI's capabilities leave out on this home (for
	// example one needing a registered device): correct, not a failure.
	Gated   bool     `json:"gated,omitempty"`
	Shown   bool     `json:"shown"`
	Settled bool     `json:"settled"`
	Errors  []string `json:"errors,omitempty"`
	Sample  []string `json:"sample,omitempty"`
}

func main() {
	home := flag.String("home", "", "agent-remote home signed in to a disposable server")
	cli := flag.String("cli", "", "agent-remote executable")
	wait := flag.Duration("wait", 20*time.Second, "how long a page may take to settle")
	flag.Parse()
	if *home == "" || *cli == "" {
		fmt.Fprintln(os.Stderr, "live-pages-probe: -home and -cli are required")
		os.Exit(2)
	}
	i18n.SetLocaleFunc(func() string { return "en" })
	a, err := app.NewWithOptions(*home, app.Options{FindExecutable: func(name string) (string, error) {
		if name == "agent-remote" {
			return *cli, nil
		}
		return exec.LookPath(name)
	}})
	if err != nil {
		fmt.Fprintln(os.Stderr, "live-pages-probe:", err)
		os.Exit(2)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1366, 768)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	tiles := []string{
		locales.SyncOpen(), locales.PortForwardTitle(), locales.ProjectDetails(),
		locales.RemoteManagement(), locales.RemoteAccounts(), locales.CredentialsTitle(), locales.SkillsTitle(),
		locales.AccountDefaultTitle(), locales.AccountImportRecoveryTitle(), locales.RuntimeRecoveryTitle(),
		locales.WireGuardNetworkTitle(), locales.DeviceTitle(), locales.EgoBrowserTitle(), locales.NodesTitle(),
		locales.MutagenEnvironmentTitle(), locales.OperationsSidebar(), locales.DiagnosticsTitle(),
		locales.DependenciesTitle(), locales.InitWizardTitle(), locales.CapabilitiesTitle(), locales.Settings(),
	}
	// What a page shows while it reads; it has settled once none shows.
	busy := []string{locales.LoadingRemote(), locales.DeviceLoading(), locales.EgoBrowserLoading(), locales.RuntimeRecoveryWorking(),
		locales.MutagenEnvironmentInspecting(), locales.AccountImportWorking()}
	// Words of a broken contract between desktop and CLI or server.
	broken := []string{"protocol", "Protocol", "schema", "unsupported response", "invalid response", "response_invalid", "Diagnostic details"}
	var pages []page
	failed := 0
	for _, tile := range tiles {
		p := page{Tile: tile}
		frame(tt)
		switch {
		case tt.HasText(locales.BackToControlCenter()):
			_ = tt.Click(locales.BackToControlCenter())
		case !tt.HasText(locales.CloseControlCenter()):
			_ = tt.Click(locales.OpenControlCenter())
		}
		frame(tt)
		for i := 0; i < 8; i++ {
			if r, ok := tt.Find(tile); ok && r.W > 0 {
				break
			}
			if sheet, ok := tt.Find(locales.ControlCenter()); ok {
				tt.Scroll(sheet.X+20, sheet.Y+120, 0, 160)
			}
			frame(tt)
		}
		if _, ok := tt.Find(tile); !ok {
			p.Gated = true
			pages = append(pages, p)
			continue
		}
		_ = tt.Click(tile)
		deadline := time.Now().Add(*wait)
		for time.Now().Before(deadline) {
			frame(tt)
			p.Shown = tt.HasText(locales.BackToControlCenter())
			p.Settled = p.Shown && !anyText(tt, busy)
			if p.Settled {
				time.Sleep(300 * time.Millisecond) // a second read a page chains
				frame(tt)
				if !anyText(tt, busy) {
					break
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
		texts := tt.Texts()
		for _, s := range texts {
			for _, w := range broken {
				if strings.Contains(s, w) {
					p.Errors = append(p.Errors, s)
					break
				}
			}
		}
		if i := index(texts, tile); i >= 0 {
			p.Sample = texts[i:min(len(texts), i+8)]
		}
		if !p.Shown || !p.Settled || len(p.Errors) > 0 {
			failed++
		}
		pages = append(pages, p)
	}
	out, _ := json.MarshalIndent(map[string]any{"probe": "live-pages", "pages": pages, "failed": failed}, "", "  ")
	fmt.Println(string(out))
	if failed > 0 {
		os.Exit(1)
	}
}

func frame(tt *ui.Tester) { tt.Frame(); tt.Advance(20 * time.Millisecond) }

func anyText(tt *ui.Tester, list []string) bool {
	for _, s := range list {
		if tt.HasText(s) {
			return true
		}
	}
	return false
}

func index(list []string, s string) int {
	for i, v := range list {
		if v == s {
			return i
		}
	}
	return -1
}
