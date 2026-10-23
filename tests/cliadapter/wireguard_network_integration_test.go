package cliadapter_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

// Runs as the explicitly selected wg binary inside a real Rust CLI subprocess.
func wireGuardDumpHelper() bool {
	if os.Getenv("DESKTOP_TEST_WG_DUMP_HELPER") != "1" || len(os.Args) != 4 || strings.Join(os.Args[1:], " ") != "show all dump" {
		return false
	}
	f, err := os.OpenFile(os.Getenv("DESKTOP_TEST_WG_DUMP_LOG"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		os.Exit(80)
	}
	fmt.Fprintln(f, strings.Join(os.Args[1:], " "))
	f.Close()
	fmt.Fprint(os.Stderr, "fixture private stderr must never become an error message")
	fmt.Fprint(os.Stdout, os.Getenv("DESKTOP_TEST_WG_DUMP"))
	if os.Getenv("DESKTOP_TEST_WG_DUMP_FAIL") == "1" {
		os.Exit(1)
	}
	os.Exit(0)
	return true
}

func TestWireGuardNetworkWithBuiltRustCLIUsesPublicMachineDump(t *testing.T) {
	binary := os.Getenv("DESKTOP_CLI_BINARY")
	if binary == "" {
		t.Skip("set DESKTOP_CLI_BINARY for real read-only WireGuard status integration")
	}
	tool := testutil.InstallControlHelper(t)
	t.Setenv("AGENT_REMOTE_WG", tool)
	t.Setenv("DESKTOP_TEST_WG_DUMP_HELPER", "1")
	log := filepath.Join(t.TempDir(), "wg.calls")
	t.Setenv("DESKTOP_TEST_WG_DUMP_LOG", log)
	client := cliadapter.Client{Binary: binary, Home: filepath.Join(t.TempDir(), "absent-home")}
	key := func(value byte) string {
		return base64.StdEncoding.EncodeToString([]byte(strings.Repeat(string([]byte{value}), 32)))
	}
	private, psk, public, peer := key(7), key(8), key(9), key(10)
	dump := fmt.Sprintf("wg-test\t%s\t%s\t51820\t0xca6c\nwg-test\t%s\t%s\t[fe80::1%%en0]:51820\t10.77.0.1/32,fd77::/64\t1700000000\t18446744073709551615\t9007199254740993\t25\n", private, public, peer, psk)
	for _, tc := range []struct{ name, dump, fail, state string }{
		{"public-fields", dump, "", "observed"},
		{"empty-success", "", "", "observed"},
		{"failed-query", dump, "1", "query_failed"},
		{"malformed-dump", "private malformed raw output\n", "", "invalid_output"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_WG_DUMP", tc.dump)
			t.Setenv("DESKTOP_TEST_WG_DUMP_FAIL", tc.fail)
			if err := os.WriteFile(log, nil, 0600); err != nil {
				t.Fatal(err)
			}
			report, err := remote.InspectWireGuardStatus(context.Background(), client)
			if err != nil || report.State != tc.state {
				t.Fatalf("unexpected state: %s %v", report.State, err)
			}
			encoded, err := json.Marshal(report)
			if err != nil || strings.Contains(string(encoded), private) || strings.Contains(string(encoded), psk) || strings.Contains(string(encoded), "fixture private stderr") || strings.Contains(string(encoded), "private malformed raw output") {
				t.Fatal("status exposed private output")
			}
			if tc.name == "public-fields" {
				if len(report.Interfaces) != 1 || report.Interfaces[0].FWMark != 0xca6c || len(report.Interfaces[0].Peers) != 1 {
					t.Fatal("public interface metadata lost")
				}
				p := report.Interfaces[0].Peers[0]
				if p.ReceivedBytes != math.MaxUint64 || p.SentBytes != 1<<53+1 || p.PublicKey != peer || p.Endpoint == nil || *p.Endpoint != "[fe80::1%en0]:51820" {
					t.Fatal("counter precision or endpoint scope lost")
				}
			} else if len(report.Interfaces) != 0 {
				t.Fatal("non-observation produced partial interfaces")
			}
			calls, err := os.ReadFile(log)
			if err != nil || string(calls) != "show all dump\n" {
				t.Fatal("machine query changed arguments or replayed")
			}
			if _, err := os.Stat(client.Home); !os.IsNotExist(err) {
				t.Fatal("read-only query initialized the CLI home")
			}
		})
	}
}
