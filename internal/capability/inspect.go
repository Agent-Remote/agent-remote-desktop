package capability

import (
	"context"
	"path/filepath"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

type Catalog struct {
	Executable string
	Schema     cliadapter.Schema
	Err        error
}

type Snapshot struct {
	Capabilities                cliadapter.Capabilities
	Config                      cliadapter.ConfigContext
	ProtocolError, ContextError error
	Catalogs                    []Catalog
}

// Inspect is passive: command discovery and context never inspect credentials,
// contact the server, run tools' version commands, or infer remote permissions.
func Inspect(ctx context.Context, client cliadapter.Client, launcher string) Snapshot {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	var snapshot Snapshot
	if client.Binary == "" {
		snapshot.ProtocolError = fault.CLIUnavailable
	} else {
		snapshot.ProtocolError = client.Call(ctx, "capabilities", struct{}{}, &snapshot.Capabilities)
		if snapshot.ProtocolError == nil {
			snapshot.ProtocolError = snapshot.Capabilities.Validate()
		}
	}
	if snapshot.ProtocolError == nil {
		if snapshot.Capabilities.Supports("context") {
			snapshot.ContextError = client.Call(ctx, "context", struct{}{}, &snapshot.Config)
			if snapshot.ContextError == nil && (snapshot.Config.Home == "" || filepath.Clean(snapshot.Config.Home) != filepath.Clean(client.Home)) {
				snapshot.ContextError = fault.CLIProtocol
			}
		} else {
			snapshot.ContextError = fault.CLIProtocol
		}
	}
	for _, target := range []struct{ name, path string }{{"agent-remote", client.Binary}, {"fclaude", launcher}} {
		catalog := Catalog{Executable: target.name}
		if target.path == "" {
			catalog.Err = fault.CLIUnavailable
		} else {
			catalog.Schema, catalog.Err = cliadapter.Discover(ctx, target.path, target.name)
			if catalog.Err == nil && target.name == "agent-remote" && snapshot.ProtocolError == nil && catalog.Schema.CLIVersion != snapshot.Capabilities.CLIVersion {
				catalog.Err = fault.CLIProtocol
			}
		}
		snapshot.Catalogs = append(snapshot.Catalogs, catalog)
	}
	return snapshot
}
