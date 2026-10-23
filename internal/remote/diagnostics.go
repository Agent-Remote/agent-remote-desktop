package remote

import (
	"context"
	"path/filepath"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

// DiagnosticReport describes observations, never implicit repair or authority.
// A present file and a successful version command remain distinct facts.
type DiagnosticReport struct {
	Home           string                 `json:"home"`
	Platform       string                 `json:"platform"`
	Online         bool                   `json:"online"`
	VerifyVersions bool                   `json:"verify_versions"`
	DeepCheck      bool                   `json:"deep_check"`
	ManifestSource string                 `json:"manifest_source"`
	Checks         []DiagnosticCheck      `json:"checks"`
	Dependencies   []DependencyInspection `json:"dependencies"`
}

type DiagnosticCheck struct {
	ID         string  `json:"id"`
	Status     string  `json:"status"`
	Path       *string `json:"path"`
	Suggestion string  `json:"suggestion"`
}

type DependencyInspection struct {
	Name            string  `json:"name"`
	Path            string  `json:"path"`
	Status          string  `json:"status"`
	RequiredVersion string  `json:"required_version"`
	License         string  `json:"license"`
	LicenseNotice   string  `json:"license_notice"`
	VersionStatus   string  `json:"version_status"`
	VersionOutput   *string `json:"version_output"`
	VersionExitCode *int    `json:"version_exit_code"`
}

func InspectDiagnostics(ctx context.Context, client cliadapter.Client, online, versions, deep bool) (DiagnosticReport, error) {
	return inspectDiagnostics(ctx, client, "diagnostics.inspect", online, versions, deep)
}

func InspectDependencies(ctx context.Context, client cliadapter.Client, versions bool) (DiagnosticReport, error) {
	return inspectDiagnostics(ctx, client, "dependencies.inspect", false, versions, false)
}

func inspectDiagnostics(ctx context.Context, client cliadapter.Client, method string, online, versions, deep bool) (DiagnosticReport, error) {
	var report DiagnosticReport
	var capabilities cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &capabilities); err != nil {
		return report, err
	}
	if capabilities.Validate() != nil || !capabilities.MatchesHostPlatform() {
		return report, fault.CLIProtocol
	}
	if !capabilities.Supports(method) {
		return report, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	params := map[string]bool{"verify_versions": versions}
	if method == "diagnostics.inspect" {
		params["online"] = online
		params["deep_check"] = deep
	}
	if err := client.Call(ctx, method, params, &report); err != nil {
		return DiagnosticReport{}, err
	}
	if report.Home == "" || filepath.Clean(report.Home) != filepath.Clean(client.Home) || report.Platform == "" || report.Online != online || report.VerifyVersions != versions || report.DeepCheck != deep || report.Checks == nil || report.Dependencies == nil || len(report.Dependencies) > 128 {
		return DiagnosticReport{}, fault.CLIProtocol
	}
	if report.ManifestSource != "default" && report.ManifestSource != "file" && report.ManifestSource != "unavailable" {
		return DiagnosticReport{}, fault.CLIProtocol
	}
	seen := make(map[string]bool)
	for _, check := range report.Checks {
		if check.ID == "" || seen[check.ID] || !diagnosticStatus(check.Status) {
			return DiagnosticReport{}, fault.CLIProtocol
		}
		seen[check.ID] = true
	}
	for _, dependency := range report.Dependencies {
		if dependency.Name == "" || !diagnosticStatus(dependency.Status) || !versionStatus(dependency.VersionStatus) {
			return DiagnosticReport{}, fault.CLIProtocol
		}
		if !versions && dependency.VersionStatus != "not_checked" {
			return DiagnosticReport{}, fault.CLIProtocol
		}
		if dependency.VersionStatus == "ok" && (dependency.VersionExitCode == nil || *dependency.VersionExitCode != 0 || dependency.VersionOutput == nil) {
			return DiagnosticReport{}, fault.CLIProtocol
		}
	}
	return report, nil
}

func diagnosticStatus(status string) bool {
	switch status {
	case "present", "missing", "inaccessible", "invalid", "not_checked", "reachable", "unavailable":
		return true
	}
	return false
}

func versionStatus(status string) bool {
	switch status {
	case "not_checked", "ok", "failed", "timeout", "output_limit", "unsafe_path", "unsupported", "missing", "budget_exhausted", "inaccessible":
		return true
	}
	return false
}
