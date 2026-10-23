// Package remote coordinates typed CLI operations without owning credentials.
package remote

import (
	"context"
	"path/filepath"
	"slices"
	"sort"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

type Overview struct {
	Capabilities                 cliadapter.Capabilities
	Config                       cliadapter.ConfigContext
	Accounts                     []cliadapter.Account
	Sessions                     []cliadapter.RemoteSession
	AccountsError, SessionsError error
}

// SessionListFilter is the server-side selector accepted by the CLI
// desktop-control sessions.list operation.  An empty tool or status set means
// "all"; the desktop's free-text search is applied separately to the returned
// public projection and must never be encoded here.
type SessionListFilter struct {
	ToolType string
	Statuses []string
}

// ValidateSessionListFilter mirrors the CLI boundary before a subprocess is
// started.  The server owns the set of status values, so statuses are bounded
// protocol identifiers rather than a closed local enum.
func ValidateSessionListFilter(filter SessionListFilter) error {
	if filter.ToolType != "" && !validDefaultTool(filter.ToolType) {
		return fault.CLIProtocol
	}
	if len(filter.Statuses) > 16 {
		return fault.CLIProtocol
	}
	seen := make(map[string]struct{}, len(filter.Statuses))
	for _, status := range filter.Statuses {
		if !validAccountText(status, 128) {
			return fault.CLIProtocol
		}
		if _, ok := seen[status]; ok {
			return fault.CLIProtocol
		}
		seen[status] = struct{}{}
	}
	return nil
}

func normalizeSessionListFilter(filter SessionListFilter) (SessionListFilter, error) {
	if err := ValidateSessionListFilter(filter); err != nil {
		return SessionListFilter{}, err
	}
	filter.Statuses = slices.Clone(filter.Statuses)
	sort.Strings(filter.Statuses)
	return filter, nil
}

// Load keeps the two query outcomes separate: a failed query never becomes an
// empty successful list, and configured credentials never imply authorization.
func Load(ctx context.Context, client cliadapter.Client) (Overview, error) {
	return LoadWithSessionFilter(ctx, client, SessionListFilter{})
}

// LoadWithSessionFilter loads the same authenticated overview as Load while
// passing an explicit, validated server-side sessions.list selector. Accounts
// remain unfiltered; the selector only affects the session inventory query.
func LoadWithSessionFilter(ctx context.Context, client cliadapter.Client, sessionFilter SessionListFilter) (Overview, error) {
	var err error
	sessionFilter, err = normalizeSessionListFilter(sessionFilter)
	if err != nil {
		return Overview{}, err
	}
	var snapshot Overview
	params := struct{}{}
	if err := client.Call(ctx, "capabilities", params, &snapshot.Capabilities); err != nil {
		return snapshot, err
	}
	if snapshot.Capabilities.Validate() != nil || !snapshot.Capabilities.MatchesHostPlatform() || !snapshot.Capabilities.Supports("context") {
		return snapshot, fault.CLIProtocol
	}
	if err := client.Call(ctx, "context", params, &snapshot.Config); err != nil {
		return snapshot, err
	}
	if snapshot.Config.Home == "" || filepath.Clean(snapshot.Config.Home) != filepath.Clean(client.Home) || !validDefaultContext(snapshot.Config) {
		return snapshot, fault.CLIProtocol
	}
	if snapshot.Config.ServerURL == nil || snapshot.Config.DeviceID == nil {
		return snapshot, nil
	}
	if snapshot.Capabilities.Supports("accounts.list") {
		var result struct {
			Items   []cliadapter.Account     `json:"items"`
			Context cliadapter.ConfigContext `json:"context"`
		}
		snapshot.AccountsError = client.Call(ctx, "accounts.list", params, &result)
		if snapshot.AccountsError == nil {
			if !sameContext(result.Context, snapshot.Config) {
				snapshot.AccountsError = fault.CLIContextChanged
			} else if result.Items == nil || len(result.Items) > 256 || slices.ContainsFunc(result.Items, func(item cliadapter.Account) bool { return !validRemoteAccount(item) }) {
				snapshot.AccountsError = fault.CLIProtocol
			} else {
				snapshot.Accounts = result.Items
			}
		}
	} else {
		snapshot.AccountsError = fault.CLIProtocol
	}
	if ctx.Err() != nil {
		return snapshot, ctx.Err()
	}
	if snapshot.Capabilities.Supports("sessions.list") {
		params := struct {
			ToolType string   `json:"tool_type,omitempty"`
			Statuses []string `json:"statuses,omitempty"`
		}{ToolType: sessionFilter.ToolType, Statuses: sessionFilter.Statuses}
		var result struct {
			Items   []cliadapter.RemoteSession `json:"items"`
			Context cliadapter.ConfigContext   `json:"context"`
		}
		snapshot.SessionsError = client.Call(ctx, "sessions.list", params, &result)
		if snapshot.SessionsError == nil {
			if !sameContext(result.Context, snapshot.Config) {
				snapshot.SessionsError = fault.CLIContextChanged
			} else if result.Items == nil || len(result.Items) > 256 || duplicateSessionIDs(result.Items) {
				snapshot.SessionsError = fault.CLIProtocol
			} else {
				snapshot.Sessions = result.Items
			}
		}
	} else {
		snapshot.SessionsError = fault.CLIProtocol
	}
	if ctx.Err() != nil {
		return snapshot, ctx.Err()
	}
	return snapshot, nil
}

func duplicateSessionIDs(items []cliadapter.RemoteSession) bool {
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if !validRemoteSession(item) || seen[item.ID] {
			return true
		}
		seen[item.ID] = true
	}
	return false
}

// validRemoteSession accepts the deliberately small, public projection emitted
// by the Rust CLI.  The server remains the source of truth for status values,
// so status/backend names are bounded text rather than a closed enum.  Every
// optional field is still checked for control characters and size before it can
// reach filtering, confirmation dialogs, or terminal launch arguments.
func validRemoteSession(item cliadapter.RemoteSession) bool {
	if !validDefaultTool(item.ID) || !validDefaultTool(item.Tool) || !validAccountText(item.Status, 128) || !validAccountText(item.Backend, 128) {
		return false
	}
	for _, value := range []string{item.AccountID, item.WorkspaceID, item.ProjectKey, item.NodeID} {
		if value != "" && !validAccountText(value, 8192) {
			return false
		}
	}
	for _, value := range []*string{item.LocalPath, item.RemotePath, item.RuntimeResourceID, item.StopTaskID, item.SkillFinalizationOperationID} {
		if value != nil && !validAccountText(*value, 32768) {
			return false
		}
	}
	return true
}

func validRemoteAccount(item cliadapter.Account) bool {
	if !validDefaultTool(item.ID) || !validAccountText(item.Tool, 128) || !validAccountText(item.Name, 512) || !validAccountText(item.Status, 128) || !validAccountText(item.Region, 128) {
		return false
	}
	return item.Backend == nil || validAccountText(*item.Backend, 128)
}

func sameContext(a, b cliadapter.ConfigContext) bool {
	// The home directory is part of the CLI data scope. Comparing only the
	// remote identity would allow a response read from another AGENT_REMOTE_HOME
	// to be accepted as the current account/session inventory.
	if a.Home == "" || b.Home == "" || filepath.Clean(a.Home) != filepath.Clean(b.Home) {
		return false
	}
	return a.ServerURL != nil && b.ServerURL != nil && *a.ServerURL == *b.ServerURL && a.DeviceID != nil && b.DeviceID != nil && *a.DeviceID == *b.DeviceID
}
