package platform_test

import (
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/platform"
)

func TestRuntimeProbeIsExplicitlyHostScoped(t *testing.T) {
	report := platform.ProbeRuntime()
	if report.SchemaVersion != 1 || report.Probe == "" {
		t.Fatalf("unexpected probe identity: %+v", report)
	}
	if !report.HostRuntime || report.CrossCompilation {
		t.Fatalf("probe must identify host execution: %+v", report)
	}
	if report.GOOS != runtime.GOOS || report.GOARCH != runtime.GOARCH {
		t.Fatalf("probe target = %s/%s, host = %s/%s", report.GOOS, report.GOARCH, runtime.GOOS, runtime.GOARCH)
	}
	if report.Platform == "freebsd" {
		t.Fatal("FreeBSD must not be reported as a supported desktop platform")
	}
	if len(report.Requirements) < 3 {
		t.Fatalf("probe omitted required observations: %+v", report.Requirements)
	}
	seen := map[string]bool{}
	for _, item := range report.Requirements {
		if item.ID == "" || seen[item.ID] {
			t.Fatalf("duplicate or empty requirement ID: %+v", report.Requirements)
		}
		seen[item.ID] = true
		switch item.Status {
		case "pass", "fail", "unknown":
		default:
			t.Fatalf("invalid requirement status %q", item.Status)
		}
	}
	terminal, ok := findRequirement(report.Requirements, "terminal")
	if !ok {
		t.Fatal("probe omitted the native terminal requirement")
	}
	if !terminal.Required {
		t.Fatalf("native terminal requirement must gate host support: %+v", terminal)
	}
}

func TestRuntimeProbeOnlyClaimsSupportedWhenRequiredChecksPass(t *testing.T) {
	report := platform.ProbeRuntime()
	for _, item := range report.Requirements {
		if item.Required && item.Status != "pass" && report.Supported {
			t.Fatalf("supported report contains failed required check: %+v", item)
		}
		if (item.ID == "webkitgtk" || item.ID == "webview2") && item.Required {
			t.Fatalf("lazy WebView runtime must remain an observation for native-content desktop: %+v", item)
		}
	}
}

func findRequirement(items []platform.Requirement, id string) (platform.Requirement, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
	}
	return platform.Requirement{}, false
}
