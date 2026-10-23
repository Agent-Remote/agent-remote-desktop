package cliadapter_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestWireGuardNetworkRejectsInvalidScopeWithoutInvokingCLI(t *testing.T) {
	client, log := wireGuardClient(t)
	relative := "relative.conf"
	for _, input := range []remote.WireGuardCheckInput{{ExpectedHome: "relative"}, {ExpectedHome: filepath.Join(client.Home, "other")}, {ExpectedHome: client.Home, Config: &relative}} {
		if _, err := remote.CheckWireGuard(context.Background(), client, input); err == nil {
			t.Fatal("invalid inspection scope was accepted")
		}
	}
	client.Home = "relative"
	if _, err := remote.InspectWireGuardStatus(context.Background(), client); err == nil {
		t.Fatal("relative status home was accepted")
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("invalid scope invoked the CLI")
	}
}

func TestWireGuardNetworkValidatesReadAndCredentialCapabilities(t *testing.T) {
	client, log := wireGuardClient(t)
	for _, mode := range []string{"wg-network-unsupported", "wg-network-write", "wg-network-credential", "other-platform", "duplicate-capability"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			if err := os.WriteFile(log, nil, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := remote.CheckWireGuard(context.Background(), client, remote.WireGuardCheckInput{ExpectedHome: client.Home}); err == nil {
				t.Fatal("unsupported configuration query ran")
			}
			if _, err := remote.InspectWireGuardStatus(context.Background(), client); err == nil {
				t.Fatal("unsupported status query ran")
			}
			calls, _ := os.ReadFile(log)
			if string(calls) != "capabilities\ncapabilities\n" {
				t.Fatalf("unexpected operation after rejected capability: %s", calls)
			}
		})
	}
}

func networkJSONMap(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestWireGuardNetworkRejectsMalformedConfigSummaries(t *testing.T) {
	client, _ := wireGuardClient(t)
	valid, err := remote.CheckWireGuard(context.Background(), client, remote.WireGuardCheckInput{ExpectedHome: client.Home})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(map[string]any)
	}{
		{"wrong-path", func(config map[string]any) { config["path"] = filepath.Join(client.Home, "other.conf") }},
		{"missing-peer-count", func(config map[string]any) { delete(config, "peer_count") }},
		{"null-addresses", func(config map[string]any) { config["addresses"] = nil }},
		{"empty-ready-addresses", func(config map[string]any) { config["addresses"] = []string{} }},
		{"injected-address", func(config map[string]any) { config["addresses"] = []string{"10.77.0.2\nPostUp=bad"} }},
		{"invalid-dns", func(config map[string]any) { config["dns"] = []string{"example.invalid"} }},
		{"oversized-peers", func(config map[string]any) { config["peer_count"] = 1025 }},
		{"invalid-mtu", func(config map[string]any) { config["mtu"] = 575 }},
		{"missing-with-data", func(config map[string]any) { config["state"] = "missing" }},
		{"unsupported-without-directives", func(config map[string]any) { config["state"] = "unsupported" }},
		{"directive-value-leak", func(config map[string]any) {
			config["unsupported_directives"] = []string{"PostUp = confidential command"}
		}},
		{"unexpected-private-field", func(config map[string]any) { config["private_key"] = "must never be accepted" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := networkJSONMap(t, valid.Config)
			tc.edit(config)
			patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_CHECK_PATCH", map[string]any{"config": config})
			if _, err := remote.CheckWireGuard(context.Background(), client, remote.WireGuardCheckInput{ExpectedHome: client.Home}); !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("malformed configuration summary accepted: %v", err)
			}
		})
	}
}

func TestWireGuardNetworkRejectsMalformedStatusAndKeepsFullCounters(t *testing.T) {
	client, _ := wireGuardClient(t)
	valid, err := remote.InspectWireGuardStatus(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(map[string]any, map[string]any)
	}{
		{"missing-received", func(_ map[string]any, peer map[string]any) { delete(peer, "received_bytes") }},
		{"null-sent", func(_ map[string]any, peer map[string]any) { peer["sent_bytes"] = nil }},
		{"negative-counter", func(_ map[string]any, peer map[string]any) { peer["received_bytes"] = -1 }},
		{"fractional-counter", func(_ map[string]any, peer map[string]any) { peer["received_bytes"] = 1.5 }},
		{"overflow-counter", func(_ map[string]any, peer map[string]any) {
			peer["received_bytes"] = json.Number("18446744073709551616")
		}},
		{"missing-public-key", func(iface map[string]any, _ map[string]any) { delete(iface, "public_key") }},
		{"null-listen-port", func(iface map[string]any, _ map[string]any) { iface["listen_port"] = nil }},
		{"overflow-port", func(iface map[string]any, _ map[string]any) { iface["listen_port"] = 65536 }},
		{"bad-peer-key", func(_ map[string]any, peer map[string]any) { peer["public_key"] = "not-a-public-key" }},
		{"secret-field", func(_ map[string]any, peer map[string]any) { peer["preshared_key"] = "forbidden" }},
		{"invalid-endpoint", func(_ map[string]any, peer map[string]any) { peer["endpoint"] = "https://user:password@host:51820" }},
		{"invalid-endpoint-scope", func(_ map[string]any, peer map[string]any) { peer["endpoint"] = "[fe80::1%en/0]:51820" }},
		{"trailing-dot-endpoint", func(_ map[string]any, peer map[string]any) { peer["endpoint"] = "host.example.:51820" }},
		{"invalid-network", func(_ map[string]any, peer map[string]any) { peer["allowed_ips"] = []string{"10.77.0.1/99"} }},
		{"future-overflow", func(_ map[string]any, peer map[string]any) { peer["latest_handshake"] = uint64(253402300800) }},
		{"control-interface", func(iface map[string]any, _ map[string]any) { iface["name"] = "wg0\nother" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			iface := networkJSONMap(t, valid.Interfaces[0])
			peer := networkJSONMap(t, valid.Interfaces[0].Peers[0])
			tc.edit(iface, peer)
			iface["peers"] = []any{peer}
			patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_STATUS_PATCH", map[string]any{"interfaces": []any{iface}})
			if _, err := remote.InspectWireGuardStatus(context.Background(), client); !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("invalid status accepted: %v", err)
			}
		})
	}
	iface := networkJSONMap(t, valid.Interfaces[0])
	peer := networkJSONMap(t, valid.Interfaces[0].Peers[0])
	peer["received_bytes"], peer["sent_bytes"] = uint64(math.MaxUint64), uint64(1<<53+1)
	iface["peers"] = []any{peer}
	patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_STATUS_PATCH", map[string]any{"interfaces": []any{iface}})
	report, err := remote.InspectWireGuardStatus(context.Background(), client)
	if err != nil || report.Interfaces[0].Peers[0].ReceivedBytes != math.MaxUint64 || report.Interfaces[0].Peers[0].SentBytes != 1<<53+1 {
		t.Fatalf("counter precision was lost: %v", err)
	}
}

func TestWireGuardNetworkEmptyAndFailedObservationsRemainDistinct(t *testing.T) {
	client, log := wireGuardClient(t)
	for _, tc := range []struct{ mode, state string }{{"wg-network-empty", "observed"}, {"wg-network-failed", "query_failed"}} {
		t.Run(tc.mode, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", tc.mode)
			if err := os.WriteFile(log, nil, 0600); err != nil {
				t.Fatal(err)
			}
			report, err := remote.InspectWireGuardStatus(context.Background(), client)
			if err != nil || report.State != tc.state || len(report.Interfaces) != 0 {
				t.Fatalf("observation semantics lost: %+v %v", report, err)
			}
			calls, _ := os.ReadFile(log)
			if string(calls) != "capabilities\nwireguard.status\n" {
				t.Fatalf("query replayed or broadened: %s", calls)
			}
		})
	}
}

func TestWireGuardNetworkFreezesConfigBeforeAwaitingCapabilities(t *testing.T) {
	client, _ := wireGuardClient(t)
	directory := t.TempDir()
	started, release := filepath.Join(directory, "started"), filepath.Join(directory, "release")
	t.Setenv("DESKTOP_TEST_CAPABILITIES_STARTED", started)
	t.Setenv("DESKTOP_TEST_CAPABILITIES_RELEASE", release)
	path := filepath.Join(directory, "reviewed.conf")
	want := path
	done := make(chan error, 1)
	go func() {
		report, err := remote.CheckWireGuard(context.Background(), client, remote.WireGuardCheckInput{ExpectedHome: client.Home, Config: &path})
		if err == nil && report.Config.Path != want {
			err = errors.New("mutable path replaced requested scope")
		}
		done <- err
	}()
	waitWireGuardBarrier(t, started)
	capture := filepath.Join(directory, "check-request")
	// The child has started after the worker's Environ snapshot. Updating the
	// capture setting uses the same environment lock, ordering caller mutation
	// after that snapshot instead of relying only on an external file signal.
	t.Setenv("DESKTOP_TEST_WIREGUARD_CHECK", capture)
	path = filepath.Join(directory, "changed.conf")
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("configuration query did not finish")
	}
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var sent remote.WireGuardCheckInput
	if json.Unmarshal(data, &sent) != nil || sent.Config == nil || *sent.Config != want {
		t.Fatal("caller edits changed the captured configuration query")
	}
}

func TestWireGuardNetworkCancellationDoesNotReplay(t *testing.T) {
	client, log := wireGuardClient(t)
	directory := t.TempDir()
	started := filepath.Join(directory, "started")
	t.Setenv("DESKTOP_TEST_WIREGUARD_STATUS_STARTED", started)
	t.Setenv("DESKTOP_TEST_WIREGUARD_STATUS_RELEASE", filepath.Join(directory, "release"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := remote.InspectWireGuardStatus(ctx, client)
		done <- err
	}()
	waitWireGuardBarrier(t, started)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation lost: %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("status worker survived cancellation")
	}
	calls, _ := os.ReadFile(log)
	if string(calls) != "capabilities\nwireguard.status\n" {
		t.Fatalf("canceled status was replayed: %s", calls)
	}
}
