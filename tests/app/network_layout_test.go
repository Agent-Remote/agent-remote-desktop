package app_test

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestNetworkFormatsExactCountersAndLocalHandshakeTime(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		t.Run(language, func(t *testing.T) {
			tt, _, _, _ := networkTester(t, "wg-network-format", language)
			if err := tt.Click(locales.WireGuardReadStatus()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, "wg-desktop")
			counter := i18n.FormatNumber(language, ^uint64(0), i18n.NumberOptions{Integer: true})
			handshake := i18n.FormatDate(language, time.Unix(1700000000, 0).In(time.Local), i18n.DateOptions{Date: i18n.MediumStyle, Time: i18n.LongStyle})
			for _, value := range []string{counter, handshake} {
				if !tt.HasText(value) {
					t.Errorf("missing exact localized value %q", value)
				}
			}
		})
	}
}

func TestNetworkMinimumLocalizedLayout(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			tt, _, _, _ := networkTester(t, "", language)
			assertWireGuardControls(t, tt, locales.WireGuardNetworkTitle(), locales.WireGuardCheckConfig(), locales.WireGuardReadStatus(), locales.ReturnDiagnostics())
			renderLocalSetup(t, tt, "network-form-"+language)
			if err := tt.Click(locales.WireGuardCheckConfig()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.WireGuardConfigState("ready"))
			assertWireGuardControls(t, tt, locales.WireGuardCheckConfig(), locales.WireGuardReadStatus(), locales.ReturnDiagnostics(), locales.WireGuardConfigState("ready"))
			renderLocalSetup(t, tt, "network-check-"+language)
			if err := tt.Click(locales.WireGuardReadStatus()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, "wg-desktop")
			assertWireGuardControls(t, tt, locales.WireGuardCheckConfig(), locales.WireGuardReadStatus(), locales.ReturnDiagnostics(), locales.WireGuardStatusState("observed"))
			renderLocalSetup(t, tt, "network-status-"+language)
			if !tt.HasText(locales.WireGuardProcessPrivilegeHelp()) {
				t.Fatal("permission observation missing its scope")
			}
			scrollNetworkTo(t, tt, locales.WireGuardNetworkField("latest_handshake"))
			renderLocalSetup(t, tt, "network-peer-"+language)
			tt.Scroll(650, 350, 0, 10000)
			renderLocalSetup(t, tt, "network-tools-"+language)
		})
	}
}

func TestNetworkPeersAndInterfacesArePaged(t *testing.T) {
	tt, _, _, _ := networkTester(t, "wg-network-many", "en")
	if err := tt.Click(locales.WireGuardReadStatus()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "wg-desktop")
	key := func(index byte) string {
		raw := make([]byte, 32)
		raw[0] = index
		return base64.StdEncoding.EncodeToString(raw)
	}
	if !tt.HasText(key(1)) || tt.HasText(key(6)) {
		t.Fatal("peer list was not bounded to one page")
	}
	scrollNetworkTo(t, tt, locales.WireGuardNextPage())
	if err := tt.Click(locales.WireGuardNextPage()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(key(6)) || tt.HasText(key(1)) || tt.HasText(key(11)) {
		t.Fatal("next peer page did not replace displayed peers")
	}
	scrollNetworkTo(t, tt, locales.WireGuardNextInterface())
	if err := tt.Click(locales.WireGuardNextInterface()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("wg-extra-1") || !tt.HasText(locales.WireGuardNoPeers()) || tt.HasText(key(6)) {
		t.Fatal("interface paging mixed old peers into new interface")
	}
	if err := tt.Click(locales.WireGuardPreviousInterface()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(key(1)) || !tt.HasText(locales.WireGuardPeerPage(1, 5)) {
		t.Fatal("switching interface failed to reset peer page")
	}
}

func scrollNetworkTo(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	for attempt := 0; attempt < 30; attempt++ {
		if rect, ok := tt.Find(label); ok {
			bottom := float32(500)
			if footer, found := tt.Find(locales.ReturnDiagnostics()); found {
				bottom = footer.Y - 24
			}
			if rect.W > 0 && rect.H > 0 && rect.Y >= 190 && rect.Y+rect.H <= bottom {
				return
			}
			delta := rect.Y + rect.H - bottom
			if rect.Y < 190 {
				delta = rect.Y - 190
			}
			if rect.H == 0 && delta == 0 {
				delta = 60
			}
			tt.Scroll(650, 350, 0, delta)
		} else {
			t.Fatalf("missing scroll target %q", label)
		}
	}
	t.Fatalf("cannot scroll to %q", label)
}
