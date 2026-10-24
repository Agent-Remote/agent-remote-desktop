package remote_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Keep the cross-repository acceptance gate honest: compiling a Rust contract
// target is useful, but it cannot prove the fixture protocol actually runs.
// This regression protects the shell entry points from silently regressing to
// `cargo test --no-run` while still allowing the ignored live upload test to
// remain opt-in.
func TestRemoteInteropScriptsExecuteRustFixtureContracts(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name     string
		script   string
		command  string
		liveNeed string
		guard    string
	}{
		{
			name:     "remote interop",
			script:   filepath.Join(root, "..", "..", "scripts", "remote-interop-acceptance.sh"),
			command:  `cargo test --test desktop_control_contract --test desktop_mutagen_environment_contract --test exact_attachment_live_contract)`,
			liveNeed: `scripts/exact-attachment-acceptance.sh --live`,
			guard:    `mutagen_probe_status="not_run"`,
		},
		{
			name:     "exact attachment",
			script:   filepath.Join(root, "..", "..", "scripts", "exact-attachment-acceptance.sh"),
			command:  `cargo test --test exact_attachment_live_contract)`,
			liveNeed: `cargo test --test exact_attachment_live_contract -- --ignored --nocapture`,
			guard:    "",
		},
	}
	for _, check := range checks {
		check := check
		t.Run(check.name, func(t *testing.T) {
			raw, err := os.ReadFile(check.script)
			if err != nil {
				t.Fatal(err)
			}
			script := string(raw)
			if !strings.Contains(script, check.command) {
				t.Fatalf("fixture command is missing or changed: %q", check.command)
			}
			if strings.Contains(script, "cargo test --no-run") {
				t.Fatal("acceptance script only compiles Rust contracts with --no-run")
			}
			if !strings.Contains(script, check.liveNeed) {
				t.Fatalf("live ignored contract invocation is missing: %q", check.liveNeed)
			}
			if check.guard != "" && !strings.Contains(script, check.guard) {
				t.Fatalf("live evidence guard is missing: %q", check.guard)
			}
		})
	}
}

func TestRemoteInteropHostPreflightCleansTemporaryHome(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(root, "..", "..", "scripts", "remote-interop-acceptance.sh"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(script)
	for _, required := range []string{
		"HOST_HOME=\"\"",
		"trap cleanup_host_home EXIT",
		"rm -rf -- \"$HOST_HOME\"",
		"AGENT_REMOTE_HOME=\"$HOST_HOME\"",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("host preflight does not clean its temporary home: %q", required)
		}
	}
}

func TestRemoteInteropExposesRealSyncChainWrapper(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	wrapper, err := os.ReadFile(filepath.Join(root, "..", "..", "scripts", "sync-chain-acceptance.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(wrapper)
	for _, required := range []string{
		"tests/linux_sync_chain_test.sh",
		"SYNC_MUTAGEN_SOURCE",
		"SYNC_CLI_BINARY",
		"real disposable Server/Node/SSH/Mutagen chain",
		"cross_compilation",
		"RESTART_RECOVERY_STATUS=passed",
		"restart_recovery_scope",
		"post_restart_persistence_and_mutation",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("sync-chain wrapper missing %q", required)
		}
	}
	interop, err := os.ReadFile(filepath.Join(root, "..", "..", "scripts", "remote-interop-acceptance.sh"))
	if err != nil {
		t.Fatal(err)
	}
	interopScript := string(interop)
	if !strings.Contains(interopScript, "--sync-chain") || !strings.Contains(interopScript, "scripts/sync-chain-acceptance.sh") {
		t.Fatal("remote interop entry point does not expose the real sync-chain wrapper")
	}
}

func TestSyncChainRestartMarkerIsFailClosed(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	wrapper, err := os.ReadFile(filepath.Join(root, "..", "..", "scripts", "sync-chain-acceptance.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(wrapper)
	for _, required := range []string{
		"restart_recovery=\"not_run\"",
		"grep -Fxq 'RESTART_RECOVERY_STATUS=passed'",
		"grep -Fxq 'RESTART_RECOVERY_STATUS=failed'",
		"restart recovery marker was absent",
		"restart_recovery=\"passed\"",
		"write_evidence failed \"$reason\" \"$log_path\" \"$host_os\" \"$host_arch\" \"$docker_arch\" \"$restart_recovery\"",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("sync-chain wrapper does not fail closed for missing restart evidence: %q", required)
		}
	}
}
