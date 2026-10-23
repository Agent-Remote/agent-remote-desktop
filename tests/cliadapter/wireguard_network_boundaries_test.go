package cliadapter_test

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestWireGuardNetworkResponseScopeBoundaries(t *testing.T) {
	client, _ := wireGuardClient(t)
	otherPlatform := "windows"
	if runtime.GOOS == "windows" {
		otherPlatform = "linux"
	}
	for _, tc := range []struct {
		name, field string
		value       any
	}{
		{"home", "home", filepath.Join(client.Home, "different")},
		{"platform", "platform", otherPlatform},
		{"elevation", "elevation", "authorized"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			set := map[string]any{tc.field: tc.value}
			patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_CHECK_PATCH", set)
			patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_STATUS_PATCH", set)
			if _, err := remote.CheckWireGuard(context.Background(), client, remote.WireGuardCheckInput{ExpectedHome: client.Home}); !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("check accepted unrelated observation: %v", err)
			}
			if _, err := remote.InspectWireGuardStatus(context.Background(), client); !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("status accepted unrelated observation: %v", err)
			}
		})
	}
}

func TestWireGuardNetworkToolAndFailureStateBoundaries(t *testing.T) {
	client, _ := wireGuardClient(t)
	valid, err := remote.CheckWireGuard(context.Background(), client, remote.WireGuardCheckInput{ExpectedHome: client.Home})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		tools []remote.WireGuardTool
	}{
		{"reordered", []remote.WireGuardTool{valid.Tools[1], valid.Tools[0]}},
		{"duplicate", []remote.WireGuardTool{valid.Tools[0], valid.Tools[0]}},
		{"incomplete", valid.Tools[:1]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_CHECK_PATCH", map[string]any{"tools": tc.tools})
			if _, err := remote.CheckWireGuard(context.Background(), client, remote.WireGuardCheckInput{ExpectedHome: client.Home}); !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("invalid platform tools accepted: %v", err)
			}
		})
	}
	for _, state := range []string{"tool_missing", "tool_not_executable", "query_failed", "timed_out", "output_too_large", "invalid_output"} {
		t.Run(state+"-with-interfaces", func(t *testing.T) {
			tool := networkJSONMap(t, valid.Tools[0])
			if state == "tool_missing" {
				tool["state"], tool["path"] = "missing", nil
			}
			if state == "tool_not_executable" {
				tool["state"] = "not_executable"
			}
			patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_STATUS_PATCH", map[string]any{"state": state, "tool": tool})
			if _, err := remote.InspectWireGuardStatus(context.Background(), client); !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("failed observation retained interfaces: %v", err)
			}
		})
	}
	for _, state := range []string{"tool_missing", "tool_not_executable"} {
		t.Run(state+"-with-available-tool", func(t *testing.T) {
			patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_STATUS_PATCH", map[string]any{"state": state, "interfaces": []any{}})
			if _, err := remote.InspectWireGuardStatus(context.Background(), client); !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("contradictory tool state accepted: %v", err)
			}
		})
	}
}

func TestWireGuardNetworkNullableFieldsMustBeExplicit(t *testing.T) {
	client, _ := wireGuardClient(t)
	check, err := remote.CheckWireGuard(context.Background(), client, remote.WireGuardCheckInput{ExpectedHome: client.Home})
	if err != nil {
		t.Fatal(err)
	}
	status, err := remote.InspectWireGuardStatus(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"mtu", "endpoint", "path"} {
		for _, missing := range []bool{false, true} {
			t.Run(field+map[bool]string{false: "-null", true: "-absent"}[missing], func(t *testing.T) {
				var queryErr error
				if field == "mtu" {
					config := networkJSONMap(t, check.Config)
					config[field] = nil
					if missing {
						delete(config, field)
					}
					patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_CHECK_PATCH", map[string]any{"config": config})
					_, queryErr = remote.CheckWireGuard(context.Background(), client, remote.WireGuardCheckInput{ExpectedHome: client.Home})
				} else {
					set := map[string]any{}
					if field == "endpoint" {
						iface := networkJSONMap(t, status.Interfaces[0])
						peer := networkJSONMap(t, status.Interfaces[0].Peers[0])
						peer[field] = nil
						if missing {
							delete(peer, field)
						}
						iface["peers"] = []any{peer}
						set["interfaces"] = []any{iface}
					} else {
						tool := map[string]any{"name": "wg", "state": "missing", "path": nil}
						if missing {
							delete(tool, field)
						}
						set["tool"], set["state"], set["interfaces"] = tool, "tool_missing", []any{}
					}
					patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_STATUS_PATCH", set)
					_, queryErr = remote.InspectWireGuardStatus(context.Background(), client)
				}
				if (!missing && queryErr != nil) || (missing && !errors.Is(queryErr, fault.CLIProtocol)) {
					t.Fatalf("nullable field presence lost: %v", queryErr)
				}
			})
		}
	}
}

func TestWireGuardNetworkRejectsDuplicateInterfacesAndPeers(t *testing.T) {
	client, _ := wireGuardClient(t)
	for _, duplicate := range []string{"interface", "peer"} {
		t.Run(duplicate, func(t *testing.T) {
			interfaces := networkBoundaryInterfaces([]int{1}, 0)
			if duplicate == "interface" {
				interfaces = append(interfaces, interfaces[0])
			} else {
				interfaces[0].Peers = append(interfaces[0].Peers, interfaces[0].Peers[0])
			}
			patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_STATUS_PATCH", map[string]any{"interfaces": interfaces})
			if _, err := remote.InspectWireGuardStatus(context.Background(), client); !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("duplicate %s accepted: %v", duplicate, err)
			}
		})
	}
}

func TestWireGuardNetworkCollectionLimitsAcceptBoundaryAndRejectOverflow(t *testing.T) {
	client, _ := wireGuardClient(t)
	for _, tc := range []struct {
		name      string
		counts    []int
		addresses int
		valid     bool
	}{
		{"64-interfaces", make([]int, 64), 0, true},
		{"65-interfaces", make([]int, 65), 0, false},
		{"1024-total-peers", []int{512, 512}, 0, true},
		{"1025-total-peers", []int{512, 513}, 0, false},
		{"256-peer-addresses", []int{1}, 256, true},
		{"257-peer-addresses", []int{1}, 257, false},
		{"4096-total-addresses", []int{8, 8}, 256, true},
		{"4097-total-addresses", []int{8, 8, 1}, 256, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			interfaces := networkBoundaryInterfaces(tc.counts, tc.addresses)
			if tc.name == "4097-total-addresses" {
				interfaces[2].Peers[0].AllowedIPs = interfaces[2].Peers[0].AllowedIPs[:1]
			}
			set := map[string]any{"interfaces": interfaces}
			patch, err := json.Marshal(map[string]any{"set": set})
			if err != nil {
				t.Fatal(err)
			}
			// Large receipts exceed Windows/Linux environment limits; do not
			// place them in the environment even temporarily.
			if len(patch) > 16*1024 {
				path := filepath.Join(t.TempDir(), "status-patch.json")
				if err := os.WriteFile(path, patch, 0600); err != nil {
					t.Fatal(err)
				}
				t.Setenv("DESKTOP_TEST_WIREGUARD_STATUS_PATCH", "")
				t.Setenv("DESKTOP_TEST_WIREGUARD_STATUS_PATCH_FILE", path)
			} else {
				patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_STATUS_PATCH", set)
			}
			report, err := remote.InspectWireGuardStatus(context.Background(), client)
			if tc.valid {
				if err != nil || len(report.Interfaces) != len(interfaces) {
					t.Fatalf("valid maximum rejected: %v", err)
				}
				for i, iface := range report.Interfaces {
					if len(iface.Peers) != len(interfaces[i].Peers) {
						t.Fatal("valid peer list truncated")
					}
					for j, peer := range iface.Peers {
						if len(peer.AllowedIPs) != len(interfaces[i].Peers[j].AllowedIPs) {
							t.Fatal("valid allowed addresses truncated")
						}
					}
				}
			} else if !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("overflow accepted: %v", err)
			}
		})
	}
}

func networkBoundaryInterfaces(peerCounts []int, addressCount int) []remote.WireGuardInterface {
	interfaces := make([]remote.WireGuardInterface, len(peerCounts))
	for i, count := range peerCounts {
		interfaces[i] = remote.WireGuardInterface{Name: fmt.Sprintf("wg%d", i), Peers: make([]remote.WireGuardPeer, count)}
		for j := range interfaces[i].Peers {
			key := make([]byte, 32)
			binary.BigEndian.PutUint32(key, uint32(j+1))
			peer := remote.WireGuardPeer{PublicKey: base64.StdEncoding.EncodeToString(key), AllowedIPs: make([]string, addressCount)}
			for k := range peer.AllowedIPs {
				peer.AllowedIPs[k] = fmt.Sprintf("fd00::%x/128", k+1)
			}
			interfaces[i].Peers[j] = peer
		}
	}
	return interfaces
}
