package app_test

import (
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/locales"
)

func TestExactRemoteAttachShowsTextOnlyAttachmentStatus(t *testing.T) {
	a, tt, _, _, _ := attachHistoryTester(t, "en", "", nil)
	defer a.Close()
	if err := openPage(tt, locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, attachSessionID)
	scrollRemoteTo(t, tt, locales.ConnectRemoteSession())
	if err := tt.Click(locales.ConnectRemoteSession()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.ConfirmRemoteConnection())
	if !tt.HasText(locales.RemoteAttachAttachmentStatusText("1")) {
		t.Fatal("exact remote confirmation did not disclose text-only attachment status")
	}
}

func TestExactRemoteAttachShowsTransferAttachmentStatus(t *testing.T) {
	a, tt, _, _, _ := attachHistoryTester(t, "en", "remote-attach-transfer", nil)
	defer a.Close()
	if err := openPage(tt, locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, attachSessionID)
	scrollRemoteTo(t, tt, locales.ConnectRemoteSession())
	if err := tt.Click(locales.ConnectRemoteSession()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.ConfirmRemoteConnection())
	if !tt.HasText(locales.RemoteAttachAttachmentStatusTransfer("1")) {
		t.Fatal("exact remote confirmation did not disclose the identity-bound transfer status")
	}
	if tt.HasText(locales.RemoteAttachAttachmentStatusText("1")) {
		t.Fatal("transfer-capable remote confirmation still advertised text-only attachments")
	}
}
