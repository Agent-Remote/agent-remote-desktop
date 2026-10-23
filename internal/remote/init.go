package remote

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

// InitContext reads the current CLI context after local setup. It is kept
// separate from Overview because initialization must freeze the exact context
// before submitting credentials; an empty context is a valid, unauthenticated
// state and is never treated as authorization.
func InitContext(ctx context.Context, client cliadapter.Client) (cliadapter.ConfigContext, error) {
	var current cliadapter.ConfigContext
	if err := client.Call(ctx, "context", struct{}{}, &current); err != nil {
		return current, err
	}
	if filepath.Clean(current.Home) != filepath.Clean(client.Home) || current.Home == "" || !validDefaultContext(current) {
		return cliadapter.ConfigContext{}, fault.CLIProtocol
	}
	return current, nil
}

// InitReadiness is the final, read-only projection shown by the initialization
// wizard. Query errors remain separate so a missing account/session list can
// never be presented as an empty successful result.
type InitReadiness struct {
	Overview Overview `json:"overview"`
	Ready    bool     `json:"ready"`
}

// AssessInitReadiness loads the shared configuration and remote lists after
// all requested mutations have completed. It does not create sessions or
// change synchronization/network state. A configured server and device are
// required for ready; account/session query failures are surfaced in Overview.
func AssessInitReadiness(ctx context.Context, client cliadapter.Client) (InitReadiness, error) {
	overview, err := Load(ctx, client)
	if err != nil {
		return InitReadiness{}, err
	}
	ready := overview.Config.ServerURL != nil && strings.TrimSpace(*overview.Config.ServerURL) != "" && overview.Config.DeviceID != nil && strings.TrimSpace(*overview.Config.DeviceID) != "" &&
		overview.AccountsError == nil && overview.SessionsError == nil
	return InitReadiness{Overview: overview, Ready: ready}, nil
}
