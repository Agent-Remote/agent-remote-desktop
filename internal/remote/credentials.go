package remote

import (
	"context"
	"path/filepath"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const CredentialsListMethod = "credentials.list"

// CredentialProfiles is the bounded, secret-free inventory returned by the
// desktop control protocol. Mutations use the separate reviewed contract in
// credentials_mutation.go and never carry device credentials.
type CredentialProfiles struct {
	Context cliadapter.ConfigContext
	Items   []cliadapter.CredentialProfile
}

func CredentialsListSupported(c cliadapter.Capabilities) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, op := range c.Operations {
		if op.Method == CredentialsListMethod {
			return op.Effect == "read" && op.Credential == "device" && op.Confirmation == ""
		}
	}
	return false
}

// ListCredentialProfiles re-checks the capability and exact configuration
// context before accepting the list. A missing or changed context is never
// represented as an empty successful inventory.
func ListCredentialProfiles(ctx context.Context, client cliadapter.Client, expected cliadapter.ConfigContext) (CredentialProfiles, error) {
	if filepath.Clean(expected.Home) != filepath.Clean(client.Home) || expected.ServerURL == nil || expected.DeviceID == nil {
		return CredentialProfiles{}, fault.CLIContextChanged
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return CredentialProfiles{}, err
	}
	if !CredentialsListSupported(caps) {
		return CredentialProfiles{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var result struct {
		Items   []cliadapter.CredentialProfile `json:"items"`
		Context cliadapter.ConfigContext       `json:"context"`
	}
	if err := client.Call(ctx, CredentialsListMethod, struct{}{}, &result); err != nil {
		return CredentialProfiles{}, err
	}
	if !sameContext(result.Context, expected) || result.Items == nil {
		return CredentialProfiles{}, fault.CLIProtocol
	}
	seen := make(map[string]struct{}, len(result.Items))
	for _, item := range result.Items {
		if item.ID == "" || item.DisplayName == "" || item.Status == "" || item.GitHubCLIMode == "" || item.SSHMode == "" || item.CreatedAt == "" || item.UpdatedAt == "" {
			return CredentialProfiles{}, fault.CLIProtocol
		}
		if _, ok := seen[item.ID]; ok {
			return CredentialProfiles{}, fault.CLIProtocol
		}
		seen[item.ID] = struct{}{}
	}
	return CredentialProfiles{Context: result.Context, Items: result.Items}, nil
}
