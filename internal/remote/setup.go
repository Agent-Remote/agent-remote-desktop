package remote

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

type LocalSetupInput struct {
	Confirmed    bool   `json:"confirmed"`
	ExpectedHome string `json:"expected_home"`
}

type LocalSetupStep struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Status string `json:"status"`
	Code   string `json:"code"`
}

type LocalSetupReceipt struct {
	OperationID string           `json:"operation_id"`
	Home        string           `json:"home"`
	Complete    bool             `json:"complete"`
	Steps       []LocalSetupStep `json:"steps"`
}

// SetupLocal performs one explicitly confirmed local mutation. Capabilities are
// checked again at execution time; an incomplete or lost receipt is never retried.
func SetupLocal(ctx context.Context, client cliadapter.Client, input LocalSetupInput) (LocalSetupReceipt, error) {
	if !input.Confirmed {
		return LocalSetupReceipt{}, fault.LocalSetupReview
	}
	if !filepath.IsAbs(input.ExpectedHome) || filepath.Clean(input.ExpectedHome) != filepath.Clean(client.Home) {
		return LocalSetupReceipt{}, fault.CLIContextChanged
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return LocalSetupReceipt{}, err
	}
	if caps.Validate() != nil || !caps.MatchesHostPlatform() {
		return LocalSetupReceipt{}, fault.CLIProtocol
	}
	if !caps.SupportsEffect("setup.local", "write") {
		return LocalSetupReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var wire struct {
		OperationID string           `json:"operation_id"`
		Home        string           `json:"home"`
		Complete    *bool            `json:"complete"`
		Steps       []LocalSetupStep `json:"steps"`
	}
	if err := client.Call(ctx, "setup.local", input, &wire); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			switch operation.Code {
			case "invalid_params", "confirmation_required":
				return LocalSetupReceipt{}, fault.LocalSetupReview
			case "configuration_changed", "configuration_unavailable", "unsupported_method", "unsupported_version":
				return LocalSetupReceipt{}, err
			}
		}
		return LocalSetupReceipt{}, fault.LocalSetupUnknown
	}
	if wire.Complete == nil || wire.OperationID == "" || wire.Home != input.ExpectedHome {
		return LocalSetupReceipt{}, fault.LocalSetupUnknown
	}
	paths := []struct{ id, relative string }{
		{"home", ""}, {"bin", "bin"}, {"dependencies", "dependencies"},
		{"secrets", "secrets"}, {"wireguard", "wireguard"}, {"ssh", "ssh"},
		{"database", "state.sqlite3"}, {"manifest", filepath.Join("dependencies", "manifest.json")},
	}
	if len(wire.Steps) != len(paths) {
		return LocalSetupReceipt{}, fault.LocalSetupUnknown
	}
	complete := true
	for i, step := range wire.Steps {
		if step.ID != paths[i].id || step.Path != filepath.Join(input.ExpectedHome, paths[i].relative) || !validSetupStep(step) {
			return LocalSetupReceipt{}, fault.LocalSetupUnknown
		}
		complete = complete && (step.Status == "ready" || step.Status == "applied")
	}
	if complete != *wire.Complete {
		return LocalSetupReceipt{}, fault.LocalSetupUnknown
	}
	return LocalSetupReceipt{OperationID: wire.OperationID, Home: wire.Home, Complete: complete, Steps: wire.Steps}, nil
}

func validSetupStep(step LocalSetupStep) bool {
	switch step.Status {
	case "ready", "applied":
		return step.Code == "none"
	case "skipped":
		return step.Code == "dependency_failed"
	case "failed":
		switch step.Code {
		case "path_invalid", "state_unavailable", "unsupported_state_version", "manifest_invalid", "local_io":
			return true
		}
	}
	return false
}
