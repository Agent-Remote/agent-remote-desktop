package remote

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

// SkillsStateDiffMethod is the bounded read-only projection used by the
// Skills page. The CLI keeps manifests and file contents behind its API
// boundary; the desktop receives changed paths and portable entry metadata.
const SkillsStateDiffMethod = "skills.state.diff"

type SkillStateDiffInput struct {
	AccountID    string
	Scope        string
	Skill        *string
	CheckpointID *string
	Limit        uint16
	Cursor       *string
}

type SkillDiffEntry struct {
	Path        string `json:"path"`
	Kind        string `json:"kind"`
	Mode        uint32 `json:"mode"`
	Size        uint64 `json:"size"`
	SHA256      string `json:"sha256"`
	ContentKind string `json:"content_kind"`
}

type SkillDiffItem struct {
	Path    string          `json:"path"`
	Base    *SkillDiffEntry `json:"base"`
	Current *SkillDiffEntry `json:"current"`
}

type SkillStateDiff struct {
	Context           cliadapter.ConfigContext `json:"context"`
	Mode              string                   `json:"mode"`
	CheckpointID      string                   `json:"checkpoint_id"`
	BaseKind          string                   `json:"base_kind"`
	BaseReferenceID   string                   `json:"base_reference_id"`
	BaseTreeDigest    string                   `json:"base_tree_digest"`
	CurrentTreeDigest string                   `json:"current_tree_digest"`
	Items             []SkillDiffItem          `json:"items"`
	NextCursor        *string                  `json:"next_cursor"`
}

func SkillsStateDiffSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, SkillsStateDiffMethod, "read", "user", "")
}

// DiffSkillState loads one current or immutable-checkpoint diff. The selector
// and checkpoint forms are mutually exclusive and are validated before the
// request reaches the CLI, preventing an ambiguous context from crossing the
// desktop boundary.
func DiffSkillState(ctx context.Context, client cliadapter.Client, expected cliadapter.ConfigContext, input SkillStateDiffInput) (SkillStateDiff, error) {
	input = copySkillStateDiffInput(input)
	if !sameContextHome(expected, client.Home) || input.Limit < 1 || input.Limit > 500 {
		return SkillStateDiff{}, fault.CLIProtocol
	}
	checkpointMode := input.CheckpointID != nil
	if checkpointMode {
		if input.AccountID != "" || input.Scope != "" || input.Skill != nil || !canonicalUUID(*input.CheckpointID) {
			return SkillStateDiff{}, fault.CLIProtocol
		}
	} else if !canonicalUUID(input.AccountID) || (input.Scope != "item" && input.Scope != "account-directory") || input.Scope == "item" && input.Skill == nil || input.Scope == "account-directory" && input.Skill != nil || input.Skill != nil && !canonicalUUID(*input.Skill) {
		return SkillStateDiff{}, fault.CLIProtocol
	}
	if !checkpointMode && input.Cursor != nil {
		// The current-head API is a single bounded observation; only immutable
		// checkpoint diffs expose an opaque continuation cursor.
		return SkillStateDiff{}, fault.CLIProtocol
	}
	if input.Cursor != nil && (len(*input.Cursor) == 0 || len(*input.Cursor) > 16384 || strings.ContainsAny(*input.Cursor, "\x00\r\n")) {
		return SkillStateDiff{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return SkillStateDiff{}, err
	}
	if !SkillsStateDiffSupported(caps) {
		return SkillStateDiff{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	params := map[string]any{"limit": input.Limit}
	if checkpointMode {
		params["checkpoint_id"] = *input.CheckpointID
	} else {
		params["account_id"], params["scope"] = input.AccountID, input.Scope
		if input.Skill != nil {
			params["skill"] = *input.Skill
		}
	}
	if input.Cursor != nil {
		params["cursor"] = *input.Cursor
	}
	var raw json.RawMessage
	if err := client.Call(ctx, SkillsStateDiffMethod, params, &raw); err != nil {
		return SkillStateDiff{}, err
	}
	var result SkillStateDiff
	if err := decodeStrict(raw, &result); err != nil {
		return SkillStateDiff{}, fault.CLIProtocol
	}
	if !sameSkillContext(result.Context, expected) || (result.Mode != "current" && result.Mode != "checkpoint") || result.Mode == "checkpoint" != checkpointMode || !canonicalUUID(result.CheckpointID) || !validSkillText(result.BaseKind, 64) || !validSkillText(result.BaseReferenceID, 256) || !hexDigest(result.BaseTreeDigest) || !hexDigest(result.CurrentTreeDigest) || len(result.Items) > int(input.Limit) {
		return SkillStateDiff{}, fault.CLIProtocol
	}
	previous := ""
	for _, item := range result.Items {
		if !validDiffPath(item.Path) || item.Path <= previous {
			return SkillStateDiff{}, fault.CLIProtocol
		}
		previous = item.Path
		if item.Base == nil && item.Current == nil || item.Base != nil && item.Base.Path != item.Path || item.Current != nil && item.Current.Path != item.Path {
			return SkillStateDiff{}, fault.CLIProtocol
		}
		if err := validateDiffEntry(item.Base); err != nil {
			return SkillStateDiff{}, err
		}
		if err := validateDiffEntry(item.Current); err != nil {
			return SkillStateDiff{}, err
		}
	}
	if result.NextCursor != nil && !checkpointMode {
		return SkillStateDiff{}, fault.CLIProtocol
	}
	if result.NextCursor != nil && (len(*result.NextCursor) == 0 || len(*result.NextCursor) > 16384 || strings.ContainsAny(*result.NextCursor, "\x00\r\n")) {
		return SkillStateDiff{}, fault.CLIProtocol
	}
	if result.NextCursor != nil && len(result.Items) != int(input.Limit) {
		return SkillStateDiff{}, fault.CLIProtocol
	}
	return result, nil
}

func copySkillStateDiffInput(input SkillStateDiffInput) SkillStateDiffInput {
	if input.Skill != nil {
		value := *input.Skill
		input.Skill = &value
	}
	if input.CheckpointID != nil {
		value := *input.CheckpointID
		input.CheckpointID = &value
	}
	if input.Cursor != nil {
		value := *input.Cursor
		input.Cursor = &value
	}
	return input
}

func validateDiffEntry(entry *SkillDiffEntry) error {
	if entry == nil {
		return nil
	}
	if !validDiffPath(entry.Path) || (entry.Kind != "file" && entry.Kind != "directory" && entry.Kind != "symlink" && entry.Kind != "runtime_link") || len(entry.SHA256) != 0 && !hexDigest(entry.SHA256) || (entry.ContentKind != "" && entry.ContentKind != "text" && entry.ContentKind != "binary") || len(entry.ContentKind) > 16 {
		return fault.CLIProtocol
	}
	return nil
}

func validDiffPath(value string) bool {
	if value == "" || len(value) > 4096 || strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.Contains(value, "//") || strings.ContainsAny(value, "\\\x00\r\n") {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}
