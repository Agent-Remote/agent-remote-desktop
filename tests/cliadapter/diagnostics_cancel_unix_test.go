//go:build unix

package cliadapter_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestRealDiagnosticCancellationStopsVersionProcessAndItsChild(t *testing.T) {
	testRealProbeCancellation(t, false)
}

func TestRealWireGuardNetworkCancellationStopsToolAndItsChild(t *testing.T) {
	testRealProbeCancellation(t, true)
}

func testRealProbeCancellation(t *testing.T, wireGuard bool) {
	t.Helper()
	binary := os.Getenv("DESKTOP_CLI_BINARY")
	if binary == "" {
		t.Skip("set DESKTOP_CLI_BINARY to verify real CLI descendant cancellation")
	}
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "dependencies"), 0700); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\nsleep 30 &\nprintf '%s %s' \"$$\" \"$!\" > \"$AGENT_REMOTE_HOME/probe.pid\"\nwait\n"
	if err := os.WriteFile(filepath.Join(home, "bin", "fixture"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if wireGuard {
		t.Setenv("AGENT_REMOTE_WG", filepath.Join(home, "bin", "fixture"))
	}
	manifest := map[string]any{"schema_version": 1, "dependencies": []any{map[string]any{"name": "fixture", "binary": "bin/fixture", "required_version": "1", "source": "test", "license": "MIT", "license_notice": "test"}}}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "dependencies", "manifest.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		client := cliadapter.Client{Binary: binary, Home: home}
		var err error
		if wireGuard {
			_, err = remote.InspectWireGuardStatus(ctx, client)
		} else {
			_, err = remote.InspectDependencies(ctx, client, true)
		}
		done <- err
	}()
	deadline := time.Now().Add(10 * time.Second)
	var pids []int
	for time.Now().Before(deadline) {
		if bytes, err := os.ReadFile(filepath.Join(home, "probe.pid")); err == nil {
			for _, raw := range strings.Fields(string(bytes)) {
				pid, err := strconv.Atoi(raw)
				if err != nil {
					t.Fatal(err)
				}
				pids = append(pids, pid)
			}
			if len(pids) == 2 {
				break
			}
			pids = nil
		}
		select {
		case err := <-done:
			t.Fatalf("probe returned before start: %v", err)
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(pids) != 2 {
		t.Fatal("probe subprocess did not start")
	}
	// Cleanup is only a test fallback; successful cancellation must already stop
	// both independently observed processes before this cleanup runs.
	t.Cleanup(func() {
		for _, pid := range pids {
			_ = syscall.Kill(pid, syscall.SIGKILL)
		}
	})
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("unexpected cancel result: %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("cancel retained CLI")
	}
	deadline = time.Now().Add(2 * time.Second)
	for _, pid := range pids {
		for time.Now().Before(deadline) && syscall.Kill(pid, 0) == nil {
			time.Sleep(10 * time.Millisecond)
		}
		if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
			t.Errorf("probe process %d survived cancellation: %v", pid, err)
		}
	}
}
