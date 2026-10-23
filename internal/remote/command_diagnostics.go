package remote

// This file adapts the versioned projections for the human `status`,
// `doctor`, `deps status` and `ssh check` commands. The native CLI remains the
// authority for the checks; this package only accepts its machine contract.

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const commandDiagnosticsSchemaVersion = 1

type CommandDiagnosticOptions struct {
	Online         bool `json:"online"`
	DeepCheck      bool `json:"deep_check"`
	VerifyVersions bool `json:"verify_versions"`
}

// CommandDiagnosticReport is the common result for status/doctor/deps
// operations. Optional fields are operation-specific and are checked by
// Validate; keeping them typed prevents accidental presentation of raw JSON.
type CommandDiagnosticReport struct {
	SchemaVersion  int                    `json:"schema_version"`
	Operation      string                 `json:"operation"`
	Home           string                 `json:"home"`
	Platform       string                 `json:"platform"`
	Online         bool                   `json:"online"`
	VerifyVersions bool                   `json:"verify_versions"`
	DeepCheck      bool                   `json:"deep_check"`
	Checks         []DiagnosticCheck      `json:"checks"`
	Dependencies   []DependencyInspection `json:"dependencies"`
	ManifestSource string                 `json:"manifest_source"`
	Remote         *CommandRemoteStatus   `json:"remote,omitempty"`
	Setup          *CommandSetupReport    `json:"setup,omitempty"`
	Repair         *CommandRepairReport   `json:"repair,omitempty"`
}

type CommandRemoteStatus struct {
	Server CommandObservation       `json:"server"`
	Device CommandDeviceObservation `json:"device"`
}

type CommandObservation struct {
	Status string `json:"status"`
}

type CommandDeviceObservation struct {
	Status       string `json:"status"`
	DeviceID     string `json:"device_id,omitempty"`
	DeviceStatus string `json:"device_status,omitempty"`
}

type CommandSetupReport struct {
	Home     string             `json:"home"`
	Complete bool               `json:"complete"`
	Steps    []CommandSetupStep `json:"steps"`
}

type CommandSetupStep struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Status string `json:"status"`
	Code   string `json:"code"`
}

type CommandRepairReport struct {
	Complete bool `json:"complete"`
}

type SSHInspectReport struct {
	SchemaVersion int                    `json:"schema_version"`
	Operation     string                 `json:"operation"`
	Home          string                 `json:"home"`
	Platform      string                 `json:"platform"`
	SSH           SSHObservation         `json:"ssh"`
	Authorization SSHAuthorizationResult `json:"authorization"`
}

type SSHObservation struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

type SSHAuthorizationResult struct {
	Status              string  `json:"status"`
	SessionID           *string `json:"session_id,omitempty"`
	NodeID              *string `json:"node_id,omitempty"`
	AuthorizationTaskID *string `json:"authorization_task_id,omitempty"`
	ExpiresIn           *uint64 `json:"expires_in,omitempty"`
}

// UnmarshalJSON rejects fields added by an incompatible CLI. This is
// deliberately local to these new contracts; the older diagnostics report
// remains compatible with its own versioned method.
func (r *CommandDiagnosticReport) UnmarshalJSON(data []byte) error {
	type plain CommandDiagnosticReport
	var value plain
	if err := decodeStrict(data, &value); err != nil {
		return err
	}
	*r = CommandDiagnosticReport(value)
	return nil
}

func (r *SSHInspectReport) UnmarshalJSON(data []byte) error {
	type plain SSHInspectReport
	var value plain
	if err := decodeStrict(data, &value); err != nil {
		return err
	}
	*r = SSHInspectReport(value)
	return nil
}

func decodeStrict(data []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fault.CLIProtocol
	}
	return nil
}

func InspectStatus(ctx context.Context, client cliadapter.Client, online bool) (CommandDiagnosticReport, error) {
	var result CommandDiagnosticReport
	err := callCommandDiagnostic(ctx, client, "status.inspect", map[string]bool{"online": online}, &result)
	if err != nil {
		return result, err
	}
	return result, validateCommandDiagnostic(result, client.Home, "status.inspect", CommandDiagnosticOptions{Online: online})
}

func InspectDoctor(ctx context.Context, client cliadapter.Client, options CommandDiagnosticOptions) (CommandDiagnosticReport, error) {
	var result CommandDiagnosticReport
	err := callCommandDiagnostic(ctx, client, "doctor.inspect", options, &result)
	if err != nil {
		return result, err
	}
	return result, validateCommandDiagnostic(result, client.Home, "doctor.inspect", options)
}

func RepairDoctor(ctx context.Context, client cliadapter.Client, options CommandRepairOptions) (CommandDiagnosticReport, error) {
	if err := validateRepairOptions(client.Home, options); err != nil {
		return CommandDiagnosticReport{}, err
	}
	var result CommandDiagnosticReport
	err := callCommandDiagnostic(ctx, client, "doctor.repair", options, &result)
	if err != nil {
		return result, err
	}
	return result, validateCommandDiagnostic(result, client.Home, "doctor.repair", options.CommandDiagnosticOptions)
}

func InspectDependenciesCommand(ctx context.Context, client cliadapter.Client, verifyVersions bool) (CommandDiagnosticReport, error) {
	var result CommandDiagnosticReport
	err := callCommandDiagnostic(ctx, client, "deps.inspect", map[string]bool{"verify_versions": verifyVersions}, &result)
	if err != nil {
		return result, err
	}
	return result, validateCommandDiagnostic(result, client.Home, "deps.inspect", CommandDiagnosticOptions{VerifyVersions: verifyVersions})
}

func RepairDependenciesCommand(ctx context.Context, client cliadapter.Client, options CommandRepairOptions) (CommandDiagnosticReport, error) {
	if err := validateRepairOptions(client.Home, options); err != nil {
		return CommandDiagnosticReport{}, err
	}
	var result CommandDiagnosticReport
	err := callCommandDiagnostic(ctx, client, "deps.repair", options, &result)
	if err != nil {
		return result, err
	}
	return result, validateCommandDiagnostic(result, client.Home, "deps.repair", options.CommandDiagnosticOptions)
}

type CommandRepairOptions struct {
	CommandDiagnosticOptions
	Confirmed    bool   `json:"confirmed"`
	ExpectedHome string `json:"expected_home"`
}

func InspectSSH(ctx context.Context, client cliadapter.Client, sessionID, expectedServerURL, expectedDeviceID *string) (SSHInspectReport, error) {
	if (sessionID == nil) != (expectedServerURL == nil) || (sessionID == nil) != (expectedDeviceID == nil) {
		return SSHInspectReport{}, fault.CLIProtocol
	}
	if sessionID != nil && (!canonicalCommandUUID(*sessionID) || *expectedServerURL == "" || len(*expectedServerURL) > 2048 || strings.ContainsAny(*expectedServerURL, "\x00\r\n") || !canonicalCommandUUID(*expectedDeviceID)) {
		return SSHInspectReport{}, fault.CLIProtocol
	}
	params := struct {
		SessionID         *string `json:"session_id,omitempty"`
		ExpectedServerURL *string `json:"expected_server_url,omitempty"`
		ExpectedDeviceID  *string `json:"expected_device_id,omitempty"`
	}{sessionID, expectedServerURL, expectedDeviceID}
	var result SSHInspectReport
	if err := callCommandDiagnostic(ctx, client, "ssh.inspect", params, &result); err != nil {
		return result, err
	}
	return result, validateSSHInspect(result, client.Home)
}

func callCommandDiagnostic(ctx context.Context, client cliadapter.Client, method string, params, result any) error {
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return err
	}
	if caps.Validate() != nil || !caps.MatchesHostPlatform() {
		return fault.CLIProtocol
	}
	effect := "read"
	confirmation := ""
	if method == "doctor.repair" || method == "deps.repair" {
		effect = "write"
		confirmation = "exact_request"
	}
	if !supportsCommandDiagnosticContract(caps, method, effect, confirmation) {
		return &cliadapter.OperationError{Code: "unsupported_method"}
	}
	return client.Call(ctx, method, params, result)
}

// supportsCommandDiagnosticContract keeps older CLI capability responses
// compatible when optional metadata is omitted, while rejecting an explicit
// weaker credential or confirmation contract.
func supportsCommandDiagnosticContract(caps cliadapter.Capabilities, method, effect, confirmation string) bool {
	for _, operation := range caps.Operations {
		if operation.Method != method || operation.Effect != effect {
			continue
		}
		if operation.Credential != "" && operation.Credential != "none" {
			return false
		}
		if confirmation != "" && operation.Confirmation != "" && operation.Confirmation != confirmation {
			return false
		}
		return true
	}
	return false
}

func validateRepairOptions(home string, options CommandRepairOptions) error {
	if !options.Confirmed {
		return fault.LocalSetupReview
	}
	if options.ExpectedHome == "" || filepath.Clean(options.ExpectedHome) != filepath.Clean(home) {
		return fault.CLIContextChanged
	}
	return nil
}

func validateCommandDiagnostic(report CommandDiagnosticReport, home, operation string, options CommandDiagnosticOptions) error {
	if report.SchemaVersion != commandDiagnosticsSchemaVersion || report.Operation != operation || report.Home == "" || filepath.Clean(report.Home) != filepath.Clean(home) || report.Platform == "" || cliadapter.PlatformName(report.Platform) != cliadapter.PlatformName(runtime.GOOS) || report.Checks == nil || report.Dependencies == nil {
		if report.Home != "" && filepath.Clean(report.Home) != filepath.Clean(home) {
			return fault.CLIContextChanged
		}
		return fault.CLIProtocol
	}
	if report.Online != options.Online || report.DeepCheck != options.DeepCheck || report.VerifyVersions != options.VerifyVersions || !validManifestSource(report.ManifestSource) {
		return fault.CLIProtocol
	}
	if len(report.Checks) > 128 || len(report.Dependencies) > 128 {
		return fault.CLIProtocol
	}
	seenChecks := make(map[string]bool, len(report.Checks))
	for _, check := range report.Checks {
		if check.ID == "" || seenChecks[check.ID] || !diagnosticStatus(check.Status) {
			return fault.CLIProtocol
		}
		seenChecks[check.ID] = true
	}
	for _, dependency := range report.Dependencies {
		if dependency.Name == "" || !diagnosticStatus(dependency.Status) || !versionStatus(dependency.VersionStatus) {
			return fault.CLIProtocol
		}
	}
	switch operation {
	case "status.inspect":
		if report.Remote == nil || report.Setup != nil || report.Repair != nil || !validRemoteStatus(*report.Remote) {
			return fault.CLIProtocol
		}
	case "doctor.repair":
		if report.Setup == nil || report.Remote != nil || report.Repair != nil || filepath.Clean(report.Setup.Home) != filepath.Clean(report.Home) || !validSetup(*report.Setup) {
			return fault.CLIProtocol
		}
	case "deps.repair":
		if report.Repair == nil || report.Setup != nil || report.Remote != nil || !report.Repair.Complete {
			return fault.CLIProtocol
		}
	default:
		if report.Setup != nil || report.Repair != nil || report.Remote != nil {
			return fault.CLIProtocol
		}
	}
	return nil
}

func validateSSHInspect(report SSHInspectReport, home string) error {
	if report.SchemaVersion != commandDiagnosticsSchemaVersion || report.Operation != "ssh.inspect" || report.Home == "" || filepath.Clean(report.Home) != filepath.Clean(home) || report.Platform == "" || cliadapter.PlatformName(report.Platform) != cliadapter.PlatformName(runtime.GOOS) {
		if report.Home != "" && filepath.Clean(report.Home) != filepath.Clean(home) {
			return fault.CLIContextChanged
		}
		return fault.CLIProtocol
	}
	if report.SSH.Status != "available" || report.SSH.Version == "" || len(report.SSH.Version) > 512 || strings.ContainsAny(report.SSH.Version, "\x00\r\n") || !validAuthorization(*(&report.Authorization)) {
		return fault.CLIProtocol
	}
	return nil
}

func validAuthorization(value SSHAuthorizationResult) bool {
	switch value.Status {
	case "not_requested":
		return value.SessionID == nil && value.NodeID == nil && value.AuthorizationTaskID == nil && value.ExpiresIn == nil
	case "succeeded", "pending", "running", "failed", "cancelled", "expired", "unknown":
		return value.SessionID != nil && validDefaultTool(*value.SessionID) && value.NodeID != nil && validDefaultTool(*value.NodeID) && value.AuthorizationTaskID != nil && validAccountText(*value.AuthorizationTaskID, 512) && value.ExpiresIn != nil && *value.ExpiresIn > 0
	default:
		return false
	}
}

func validRemoteStatus(value CommandRemoteStatus) bool {
	for _, status := range []string{value.Server.Status, value.Device.Status} {
		switch status {
		case "reachable", "inaccessible", "unavailable", "missing", "invalid", "not_checked", "unknown", "observed":
		default:
			return false
		}
	}
	if value.Device.Status == "observed" {
		return value.Device.DeviceID != "" && value.Device.DeviceStatus != ""
	}
	return value.Device.DeviceID == "" && value.Device.DeviceStatus == ""
}

func validSetup(value CommandSetupReport) bool {
	if value.Home == "" || value.Steps == nil || len(value.Steps) > 16 {
		return false
	}
	seen := make(map[string]bool, len(value.Steps))
	for _, step := range value.Steps {
		if step.ID == "" || seen[step.ID] || step.Path == "" {
			return false
		}
		seen[step.ID] = true
		switch step.Status {
		case "ready", "applied", "failed", "skipped":
		default:
			return false
		}
		switch step.Code {
		case "none", "path_invalid", "state_unavailable", "unsupported_state_version", "manifest_invalid", "local_io", "dependency_failed":
		default:
			return false
		}
	}
	return true
}

func validManifestSource(value string) bool {
	return value == "default" || value == "file" || value == "unavailable"
}

func canonicalCommandUUID(value string) bool {
	if len(value) != 36 || value == "00000000-0000-0000-0000-000000000000" {
		return false
	}
	for index, char := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if char != '-' {
				return false
			}
			continue
		}
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return false
		}
	}
	return true
}
