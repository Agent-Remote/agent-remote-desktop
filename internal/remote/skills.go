package remote

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const (
	SkillsListMethod           = "skills.list"
	SkillsInfoMethod           = "skills.info"
	SkillsStateListMethod      = "skills.state.list"
	SkillsStateConflictsMethod = "skills.state.conflicts"
	SkillsStateInfoMethod      = "skills.state.info"
	SkillsChangeMethod         = "skills.change"
)

// SkillEffective is the safe subset of the CLI's resolved rule. Source
// metadata, package contents and retention details never cross desktop-control.
type SkillEffective struct {
	Enabled         bool    `json:"enabled"`
	RevisionID      string  `json:"revision_id"`
	Eligible        bool    `json:"eligible"`
	Included        bool    `json:"included"`
	ExclusionReason *string `json:"exclusion_reason"`
}

type SkillSummary struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Removed           bool            `json:"removed"`
	DefaultEnabled    bool            `json:"default_enabled"`
	DefaultRevisionID string          `json:"default_revision_id"`
	Effective         *SkillEffective `json:"effective"`
	ProjectDiscovery  string          `json:"project_discovery"`
	ModelLoaded       bool            `json:"model_loaded"`
	RevisionCount     int             `json:"revision_count"`
}

type LocalSkillSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AccountID   string `json:"account_id"`
	Status      string `json:"status"`
	Enabled     bool   `json:"enabled"`
	RevisionID  string `json:"revision_id"`
	ModelLoaded bool   `json:"model_loaded"`
}

type SkillCatalog struct {
	Context    cliadapter.ConfigContext
	Generation int64
	Items      []SkillSummary
	LocalItems []LocalSkillSummary
}

type SkillListOptions struct {
	Tool          string
	AccountID     string
	Effective     bool
	IncludeSystem bool
}

// SkillInfoInput is the exact selector for the dedicated library details
// view. Tool and account scopes are mutually exclusive; the identifier may be
// a canonical UUID or a lower-case library name accepted by the CLI.
type SkillInfoInput struct {
	Skill     string
	Tool      string
	AccountID string
}

type SkillInfoRevision struct {
	ID       string `json:"id"`
	Number   int64  `json:"number"`
	Retained bool   `json:"retained"`
}

type SkillInfoEffective struct {
	Enabled         bool    `json:"enabled"`
	RevisionID      string  `json:"revision_id"`
	Eligible        bool    `json:"eligible"`
	Included        bool    `json:"included"`
	ExclusionReason *string `json:"exclusion_reason"`
}

// SkillInfo is deliberately metadata-only. Source locators, tracking data,
// revision metadata, paths and content stay inside the Rust CLI/API boundary.
type SkillInfo struct {
	Kind              string              `json:"kind"`
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	Status            string              `json:"status"`
	Removed           bool                `json:"removed"`
	AccountID         string              `json:"account_id"`
	Epoch             int64               `json:"epoch"`
	DefaultEnabled    bool                `json:"default_enabled"`
	DefaultRevisionID string              `json:"default_revision_id"`
	SourceCheckpoint  string              `json:"source_checkpoint_id"`
	RevisionCount     int                 `json:"revision_count"`
	Revisions         []SkillInfoRevision `json:"revisions"`
	Effective         *SkillInfoEffective `json:"effective"`
	ProjectDiscovery  string              `json:"project_discovery"`
	ModelLoaded       bool                `json:"model_loaded"`
}

type SkillInfoResult struct {
	Context  cliadapter.ConfigContext `json:"context"`
	Selector SkillInfoInputProjection `json:"selector"`
	Item     SkillInfo                `json:"item"`
}

type SkillInfoInputProjection struct {
	Skill     string  `json:"skill"`
	Tool      *string `json:"tool"`
	AccountID *string `json:"account_id"`
}

func SkillsInfoSupported(c cliadapter.Capabilities) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, op := range c.Operations {
		if op.Method == SkillsInfoMethod {
			return op.Effect == "read" && op.Credential == "user" && op.Confirmation == ""
		}
	}
	return false
}

// InfoSkill reads one exact library/account-local Skill details projection.
// The CLI/API remains authoritative for source and content; the desktop only
// receives bounded display metadata.
func InfoSkill(ctx context.Context, client cliadapter.Client, expected cliadapter.ConfigContext, input SkillInfoInput) (SkillInfo, error) {
	if filepath.Clean(expected.Home) != filepath.Clean(client.Home) || expected.ServerURL == nil || strings.TrimSpace(*expected.ServerURL) == "" {
		return SkillInfo{}, fault.CLIContextChanged
	}
	input.Skill = strings.TrimSpace(input.Skill)
	input.Tool = strings.TrimSpace(input.Tool)
	input.AccountID = strings.TrimSpace(input.AccountID)
	if !validSkillIdentifier(input.Skill) || input.Tool != "" && input.AccountID != "" || input.Tool != "" && !validSkillText(input.Tool, 128) || input.AccountID != "" && !canonicalUUID(input.AccountID) {
		return SkillInfo{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return SkillInfo{}, err
	}
	if !SkillsInfoSupported(caps) {
		return SkillInfo{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	params := map[string]any{"skill": input.Skill}
	if input.Tool != "" {
		params["tool"] = input.Tool
	}
	if input.AccountID != "" {
		params["account_id"] = input.AccountID
	}
	var raw json.RawMessage
	if err := client.Call(ctx, SkillsInfoMethod, params, &raw); err != nil {
		return SkillInfo{}, err
	}
	var result SkillInfoResult
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&result) != nil || decoder.Decode(new(any)) != io.EOF {
		return SkillInfo{}, fault.CLIProtocol
	}
	if !sameSkillContext(result.Context, expected) || result.Selector.Skill != input.Skill {
		return SkillInfo{}, fault.CLIContextChanged
	}
	if input.Tool == "" {
		if result.Selector.Tool != nil || input.AccountID == "" && result.Selector.AccountID != nil {
			return SkillInfo{}, fault.CLIContextChanged
		}
	} else if result.Selector.Tool == nil || *result.Selector.Tool != input.Tool || result.Selector.AccountID != nil {
		return SkillInfo{}, fault.CLIContextChanged
	}
	if input.AccountID == "" {
		if result.Selector.AccountID != nil {
			return SkillInfo{}, fault.CLIContextChanged
		}
	} else if result.Selector.AccountID == nil || *result.Selector.AccountID != input.AccountID || result.Selector.Tool != nil {
		return SkillInfo{}, fault.CLIContextChanged
	}
	if err := validateSkillInfo(result.Item); err != nil {
		return SkillInfo{}, err
	}
	// UUID selectors are immutable identities; a server response for another
	// installation must never be rendered as the reviewed Skill.
	if canonicalUUID(input.Skill) && result.Item.ID != input.Skill {
		return SkillInfo{}, fault.CLIContextChanged
	}
	return result.Item, nil
}

func validSkillIdentifier(value string) bool {
	// A nil UUID is never a server Skill identity. It must not fall through to
	// the human library-name grammar below, which would otherwise treat the
	// all-zero UUID as a valid slug.
	if value == "00000000-0000-0000-0000-000000000000" {
		return false
	}
	if canonicalUUID(value) {
		return true
	}
	if value == "" || len(value) > 64 || value[0] == '-' || value[len(value)-1] == '-' {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

func validateSkillInfo(item SkillInfo) error {
	if item.Kind != "library" && item.Kind != "account_local" || !validSkillText(item.ID, 256) || !validSkillText(item.Name, 512) || item.RevisionCount < 0 || item.RevisionCount > 100000 || len(item.Revisions) > 4096 || item.RevisionCount != len(item.Revisions) {
		return fault.CLIProtocol
	}
	if item.Kind == "library" && item.AccountID != "" || item.Kind == "account_local" && !canonicalUUID(item.AccountID) {
		return fault.CLIProtocol
	}
	if item.Epoch < 0 || item.DefaultRevisionID != "" && !validSkillText(item.DefaultRevisionID, 256) || item.SourceCheckpoint != "" && !validSkillText(item.SourceCheckpoint, 256) || item.Status != "" && !validSkillText(item.Status, 128) || item.ProjectDiscovery != "" && !validSkillText(item.ProjectDiscovery, 128) {
		return fault.CLIProtocol
	}
	seen := make(map[string]struct{}, len(item.Revisions))
	for _, revision := range item.Revisions {
		if !validSkillText(revision.ID, 256) || revision.Number < 1 || revision.Number > 1000000000 {
			return fault.CLIProtocol
		}
		if _, ok := seen[revision.ID]; ok {
			return fault.CLIProtocol
		}
		seen[revision.ID] = struct{}{}
	}
	if item.Effective != nil && (!validSkillText(item.Effective.RevisionID, 256) || item.Effective.ExclusionReason != nil && !validSkillText(*item.Effective.ExclusionReason, 512)) {
		return fault.CLIProtocol
	}
	return nil
}

// SkillChangeInput is the exact reviewed request for the supported Skill rule
// mutations.  Scope is explicit and contains no source content or credentials.
type SkillChangeInput struct {
	Confirmed          bool                     `json:"confirmed"`
	Expected           cliadapter.ConfigContext `json:"expected_context"`
	ExpectedGeneration int64                    `json:"expected_generation"`
	Skill              string                   `json:"skill"`
	Action             string                   `json:"action"`
	Scope              SkillScope               `json:"scope"`
}

type SkillScope struct {
	Tools     []string `json:"tools"`
	AccountID *string  `json:"account_id"`
}

type SkillChangeReceipt struct {
	OperationID string                   `json:"operation_id"`
	Context     cliadapter.ConfigContext `json:"context"`
	Accepted    bool                     `json:"accepted"`
	Action      string                   `json:"action"`
	Skill       string                   `json:"skill"`
	Generation  int64                    `json:"generation"`
	Changed     bool                     `json:"changed"`
	Status      string                   `json:"status"`
}

// SkillConflictSummary is the bounded, metadata-only projection returned by
// the CLI. Paths, saved branch inputs, digests and resolution choices remain
// inside Rust and the Server.
type SkillConflictSummary struct {
	ID                string  `json:"id"`
	Attempt           *int64  `json:"attempt,omitempty"`
	SkillID           string  `json:"skill_id,omitempty"`
	Name              string  `json:"name,omitempty"`
	Mode              string  `json:"mode,omitempty"`
	Status            string  `json:"status"`
	Reason            *string `json:"reason,omitempty"`
	InstallationEpoch *int64  `json:"installation_epoch,omitempty"`
	TargetEpoch       *int64  `json:"target_epoch,omitempty"`
	DirectoryEpoch    *int64  `json:"directory_epoch,omitempty"`
	CreatedAt         string  `json:"created_at,omitempty"`
}

type SkillConflictPage struct {
	Items      []SkillConflictSummary `json:"items"`
	NextCursor *string                `json:"next_cursor"`
}

type SkillStateConflicts struct {
	Context      cliadapter.ConfigContext `json:"context"`
	Selector     SkillStateSelector       `json:"selector"`
	Publications SkillConflictPage        `json:"publications"`
	Migrations   SkillConflictPage        `json:"migrations"`
}

type SkillStateSelector struct {
	AccountID string  `json:"account_id"`
	Scope     string  `json:"scope"`
	Skill     *string `json:"skill"`
}

// SkillStateCheckpoint is the bounded metadata projection used by the state
// history page. Content, manifests, paths, retention diagnostics and storage
// internals remain inside the CLI/API boundary.
type SkillStateCheckpoint struct {
	ID                       string  `json:"id"`
	AccountID                string  `json:"account_id"`
	Scope                    string  `json:"scope"`
	StateID                  *string `json:"state_id"`
	SkillID                  *string `json:"skill_id"`
	Origin                   *string `json:"origin"`
	RevisionID               *string `json:"revision_id"`
	InstallationEpoch        *int64  `json:"installation_epoch"`
	StateEpoch               *int64  `json:"state_epoch"`
	DirectoryEpoch           *int64  `json:"directory_epoch"`
	CurrentStateEpoch        *int64  `json:"current_state_epoch"`
	CurrentDirectoryEpoch    *int64  `json:"current_directory_epoch"`
	ParentID                 *string `json:"parent_id"`
	ContentDigest            string  `json:"content_digest"`
	Retained                 bool    `json:"retained"`
	IsHead                   bool    `json:"is_head"`
	InvalidSkillFormat       bool    `json:"invalid_skill_format"`
	SourceSessionReferenceID *string `json:"source_session_reference_id"`
	FinalizationID           *string `json:"finalization_id"`
	FinalizationStatus       *string `json:"finalization_status"`
	StorageLocation          string  `json:"storage_location"`
	CreatedAt                string  `json:"created_at"`
}

type SkillStateCheckpointPage struct {
	Items      []SkillStateCheckpoint `json:"items"`
	NextCursor *string                `json:"next_cursor"`
}

type SkillStatePending struct {
	ID                   string `json:"id"`
	SnapshotID           string `json:"snapshot_id"`
	SessionReferenceID   string `json:"session_reference_id"`
	NodeID               string `json:"node_id"`
	IncomingDigest       string `json:"incoming_digest"`
	Status               string `json:"status"`
	StorageLocation      string `json:"storage_location"`
	ExportableFromServer bool   `json:"exportable_from_server"`
}

type SkillStatePendingPage struct {
	Items      []SkillStatePending `json:"items"`
	NextCursor *string             `json:"next_cursor"`
}

type SkillStateList struct {
	Context     cliadapter.ConfigContext `json:"context"`
	Selector    SkillStateSelector       `json:"selector"`
	Checkpoints SkillStateCheckpointPage `json:"checkpoints"`
	Pending     SkillStatePendingPage    `json:"pending"`
}

// SkillStateInfo is the metadata-only detail for one exact checkpoint. The
// CLI keeps manifests, storage diagnostics and content behind its API boundary.
type SkillStateInfo struct {
	Context    cliadapter.ConfigContext `json:"context"`
	Status     string                   `json:"status"`
	Checkpoint *SkillStateCheckpoint    `json:"checkpoint"`
}

func SkillsStateInfoSupported(c cliadapter.Capabilities) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, op := range c.Operations {
		if op.Method == SkillsStateInfoMethod {
			return op.Effect == "read" && op.Credential == "user" && op.Confirmation == ""
		}
	}
	return false
}

// InfoSkillState reads one immutable checkpoint projection. An expired
// checkpoint is a valid, explicit observation with no checkpoint payload.
func InfoSkillState(ctx context.Context, client cliadapter.Client, expected cliadapter.ConfigContext, checkpointID string) (SkillStateInfo, error) {
	checkpointID = strings.TrimSpace(checkpointID)
	if filepath.Clean(expected.Home) != filepath.Clean(client.Home) || expected.ServerURL == nil || strings.TrimSpace(*expected.ServerURL) == "" || !canonicalUUID(checkpointID) {
		return SkillStateInfo{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return SkillStateInfo{}, err
	}
	if !SkillsStateInfoSupported(caps) {
		return SkillStateInfo{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, SkillsStateInfoMethod, map[string]string{"checkpoint_id": checkpointID}, &raw); err != nil {
		return SkillStateInfo{}, err
	}
	var result SkillStateInfo
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&result) != nil || decoder.Decode(new(any)) != io.EOF || !sameSkillContext(result.Context, expected) || (result.Status != "ready" && result.Status != "state_expired") {
		return SkillStateInfo{}, fault.CLIProtocol
	}
	if result.Status == "state_expired" {
		if result.Checkpoint != nil {
			return SkillStateInfo{}, fault.CLIProtocol
		}
		return result, nil
	}
	if result.Checkpoint == nil || result.Checkpoint.ID != checkpointID {
		return SkillStateInfo{}, fault.CLIProtocol
	}
	item := *result.Checkpoint
	var skill *string
	if item.Scope == "item" {
		if item.SkillID == nil {
			return SkillStateInfo{}, fault.CLIProtocol
		}
		skill = item.SkillID
	} else if item.Scope != "account-directory" {
		return SkillStateInfo{}, fault.CLIProtocol
	}
	if err := validateSkillStateCheckpointPage(SkillStateCheckpointPage{Items: []SkillStateCheckpoint{item}}, 1, SkillStateListInput{AccountID: item.AccountID, Scope: item.Scope, Skill: skill, Limit: 1}); err != nil {
		return SkillStateInfo{}, err
	}
	return result, nil
}

type SkillStateListInput struct {
	AccountID     string
	Scope         string
	Skill         *string
	Limit         uint16
	Cursor        *string
	PendingCursor *string
}

func SkillsStateListSupported(c cliadapter.Capabilities) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, op := range c.Operations {
		if op.Method == SkillsStateListMethod {
			return op.Effect == "read" && op.Credential == "user" && op.Confirmation == ""
		}
	}
	return false
}

// ListSkillState reads one exact account/skill history selector. The two page
// cursors advance independently, matching the CLI's state list command.
func ListSkillState(ctx context.Context, client cliadapter.Client, expected cliadapter.ConfigContext, input SkillStateListInput) (SkillStateList, error) {
	input = copySkillStateListInput(input)
	if filepath.Clean(expected.Home) != filepath.Clean(client.Home) || expected.ServerURL == nil || strings.TrimSpace(*expected.ServerURL) == "" {
		return SkillStateList{}, fault.CLIContextChanged
	}
	if !canonicalUUID(input.AccountID) || (input.Scope != "item" && input.Scope != "account-directory") || input.Scope == "item" && input.Skill == nil || input.Scope == "account-directory" && input.Skill != nil || input.Skill != nil && !canonicalUUID(*input.Skill) || input.Limit < 1 || input.Limit > 200 || input.Cursor != nil && !canonicalUUID(*input.Cursor) || input.PendingCursor != nil && !canonicalUUID(*input.PendingCursor) {
		return SkillStateList{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return SkillStateList{}, err
	}
	if !SkillsStateListSupported(caps) {
		return SkillStateList{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	params := struct {
		AccountID     string  `json:"account_id"`
		Scope         string  `json:"scope"`
		Skill         *string `json:"skill,omitempty"`
		Limit         uint16  `json:"limit"`
		Cursor        *string `json:"cursor,omitempty"`
		PendingCursor *string `json:"pending_cursor,omitempty"`
	}{input.AccountID, input.Scope, input.Skill, input.Limit, input.Cursor, input.PendingCursor}
	var raw json.RawMessage
	if err := client.Call(ctx, SkillsStateListMethod, params, &raw); err != nil {
		return SkillStateList{}, err
	}
	var result SkillStateList
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&result) != nil {
		return SkillStateList{}, fault.CLIProtocol
	}
	if !sameSkillContext(result.Context, expected) || result.Selector.AccountID != input.AccountID || result.Selector.Scope != input.Scope || !sameOptional(result.Selector.Skill, input.Skill) {
		return SkillStateList{}, fault.CLIContextChanged
	}
	if err := validateSkillStateCheckpointPage(result.Checkpoints, input.Limit, input); err != nil {
		return SkillStateList{}, err
	}
	if err := validateSkillStatePendingPage(result.Pending, input.Limit, input); err != nil {
		return SkillStateList{}, err
	}
	return result, nil
}

func copySkillStateListInput(input SkillStateListInput) SkillStateListInput {
	if input.Skill != nil {
		value := *input.Skill
		input.Skill = &value
	}
	if input.Cursor != nil {
		value := *input.Cursor
		input.Cursor = &value
	}
	if input.PendingCursor != nil {
		value := *input.PendingCursor
		input.PendingCursor = &value
	}
	return input
}

func validateSkillStateCheckpointPage(page SkillStateCheckpointPage, limit uint16, input SkillStateListInput) error {
	if page.Items == nil || len(page.Items) > int(limit) {
		return fault.CLIProtocol
	}
	previous := ""
	for _, item := range page.Items {
		if !canonicalUUID(item.ID) || item.ID <= previous || item.AccountID != input.AccountID || item.Scope != input.Scope || !validSkillText(item.ContentDigest, 64) || !hexDigest(item.ContentDigest) || !validSkillText(item.StorageLocation, 16) || !validSkillText(item.CreatedAt, 128) {
			return fault.CLIProtocol
		}
		previous = item.ID
		if item.Scope == "item" {
			if item.SkillID == nil || !canonicalUUID(*item.SkillID) || item.StateID == nil || !canonicalUUID(*item.StateID) || item.RevisionID == nil || !canonicalUUID(*item.RevisionID) || item.InstallationEpoch == nil || *item.InstallationEpoch < 1 || item.Origin == nil || !skillStatus(*item.Origin, "user_library", "account_local") {
				return fault.CLIProtocol
			}
		} else if item.SkillID != nil || item.StateID != nil || item.RevisionID != nil || item.InstallationEpoch != nil || item.Origin != nil || item.StateEpoch != nil || item.CurrentStateEpoch != nil {
			return fault.CLIProtocol
		}
		for _, epoch := range []*int64{item.InstallationEpoch, item.StateEpoch, item.DirectoryEpoch, item.CurrentStateEpoch, item.CurrentDirectoryEpoch} {
			if epoch != nil && *epoch < 1 {
				return fault.CLIProtocol
			}
		}
		for _, id := range []*string{item.StateID, item.SkillID, item.RevisionID, item.ParentID, item.SourceSessionReferenceID, item.FinalizationID} {
			if id != nil && !canonicalUUID(*id) {
				return fault.CLIProtocol
			}
		}
		if item.FinalizationStatus != nil && !validSkillText(*item.FinalizationStatus, 64) {
			return fault.CLIProtocol
		}
	}
	if page.NextCursor != nil && !canonicalUUID(*page.NextCursor) {
		return fault.CLIProtocol
	}
	if page.NextCursor != nil && len(page.Items) != int(limit) {
		return fault.CLIProtocol
	}
	return nil
}

func validateSkillStatePendingPage(page SkillStatePendingPage, limit uint16, input SkillStateListInput) error {
	if page.Items == nil || len(page.Items) > int(limit) {
		return fault.CLIProtocol
	}
	previous := ""
	for _, item := range page.Items {
		if !canonicalUUID(item.ID) || item.ID <= previous || !canonicalUUID(item.SnapshotID) || !canonicalUUID(item.SessionReferenceID) || !canonicalUUID(item.NodeID) || !hexDigest(item.IncomingDigest) || item.Status != "upload_pending" || item.StorageLocation != "source_node" || item.ExportableFromServer {
			return fault.CLIProtocol
		}
		previous = item.ID
	}
	if page.NextCursor != nil && !canonicalUUID(*page.NextCursor) {
		return fault.CLIProtocol
	}
	if page.NextCursor != nil && len(page.Items) != int(limit) {
		return fault.CLIProtocol
	}
	return nil
}

func hexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

type SkillStateConflictsInput struct {
	AccountID       string
	Scope           string
	Skill           *string
	Limit           uint16
	Cursor          *string
	MigrationCursor *string
}

func SkillsStateConflictsSupported(c cliadapter.Capabilities) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, op := range c.Operations {
		if op.Method == SkillsStateConflictsMethod {
			return op.Effect == "read" && op.Credential == "user" && op.Confirmation == ""
		}
	}
	return false
}

// ListSkillConflicts performs a read-only, exact-scope query. The CLI owns
// user credentials and all remote response validation; the desktop accepts
// only the public conflict summary and page cursors.
func ListSkillConflicts(ctx context.Context, client cliadapter.Client, expected cliadapter.ConfigContext, input SkillStateConflictsInput) (SkillStateConflicts, error) {
	input = copySkillStateInput(input)
	if filepath.Clean(expected.Home) != filepath.Clean(client.Home) || expected.ServerURL == nil || strings.TrimSpace(*expected.ServerURL) == "" {
		return SkillStateConflicts{}, fault.CLIContextChanged
	}
	if !canonicalUUID(input.AccountID) || (input.Scope != "item" && input.Scope != "account-directory") || input.Scope == "item" && input.Skill == nil || input.Scope == "account-directory" && input.Skill != nil || input.Skill != nil && !canonicalUUID(*input.Skill) || input.Limit < 1 || input.Limit > 200 || input.Cursor != nil && !canonicalUUID(*input.Cursor) || input.MigrationCursor != nil && !canonicalUUID(*input.MigrationCursor) {
		return SkillStateConflicts{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return SkillStateConflicts{}, err
	}
	if !SkillsStateConflictsSupported(caps) {
		return SkillStateConflicts{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	params := struct {
		AccountID       string  `json:"account_id"`
		Scope           string  `json:"scope"`
		Skill           *string `json:"skill,omitempty"`
		Limit           uint16  `json:"limit"`
		Cursor          *string `json:"cursor,omitempty"`
		MigrationCursor *string `json:"migration_cursor,omitempty"`
	}{input.AccountID, input.Scope, input.Skill, input.Limit, input.Cursor, input.MigrationCursor}
	var raw json.RawMessage
	if err := client.Call(ctx, SkillsStateConflictsMethod, params, &raw); err != nil {
		return SkillStateConflicts{}, err
	}
	var result SkillStateConflicts
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&result) != nil {
		return SkillStateConflicts{}, fault.CLIProtocol
	}
	if !sameSkillContext(result.Context, expected) || result.Selector.AccountID != input.AccountID || result.Selector.Scope != input.Scope || !sameOptional(result.Selector.Skill, input.Skill) {
		return SkillStateConflicts{}, fault.CLIContextChanged
	}
	if err := validateConflictPage(result.Publications, input.Limit, true); err != nil {
		return SkillStateConflicts{}, err
	}
	if err := validateConflictPage(result.Migrations, input.Limit, false); err != nil {
		return SkillStateConflicts{}, err
	}
	return result, nil
}

func copySkillStateInput(input SkillStateConflictsInput) SkillStateConflictsInput {
	if input.Skill != nil {
		value := *input.Skill
		input.Skill = &value
	}
	if input.Cursor != nil {
		value := *input.Cursor
		input.Cursor = &value
	}
	if input.MigrationCursor != nil {
		value := *input.MigrationCursor
		input.MigrationCursor = &value
	}
	return input
}

func sameOptional(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func validateConflictPage(page SkillConflictPage, limit uint16, publication bool) error {
	if page.Items == nil || len(page.Items) > int(limit) {
		return fault.CLIProtocol
	}
	previous := ""
	for _, item := range page.Items {
		if !validSkillText(item.ID, 128) || item.ID <= previous || !validSkillText(item.Status, 32) {
			return fault.CLIProtocol
		}
		previous = item.ID
		if publication {
			if item.Attempt == nil || *item.Attempt < 1 || !skillStatus(item.Status, "conflicted", "superseded", "published", "detached") || item.SkillID != "" || item.Name != "" || item.Mode != "" || item.InstallationEpoch != nil || item.TargetEpoch != nil || item.DirectoryEpoch != nil || item.CreatedAt != "" {
				return fault.CLIProtocol
			}
		} else if item.SkillID == "" || !validSkillText(item.SkillID, 128) || !validSkillText(item.Name, 256) || !skillStatus(item.Status, "conflicted", "superseded", "ready") || !skillStatus(item.Mode, "initial", "forward", "older", "resume", "incremental") || item.InstallationEpoch == nil || *item.InstallationEpoch < 1 || item.TargetEpoch == nil || *item.TargetEpoch < 1 || item.DirectoryEpoch == nil || *item.DirectoryEpoch < 1 || !validSkillText(item.CreatedAt, 128) || item.Attempt != nil || item.Reason != nil {
			return fault.CLIProtocol
		}
		if item.Reason != nil && !validSkillText(*item.Reason, 512) {
			return fault.CLIProtocol
		}
	}
	if page.NextCursor != nil && !canonicalUUID(*page.NextCursor) {
		return fault.CLIProtocol
	}
	return nil
}

func skillStatus(value string, accepted ...string) bool {
	for _, candidate := range accepted {
		if value == candidate {
			return true
		}
	}
	return false
}

func SkillsChangeSupported(c cliadapter.Capabilities) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, op := range c.Operations {
		if op.Method == SkillsChangeMethod {
			return op.Effect == "write" && op.Credential == "user" && op.Confirmation == "exact_request"
		}
	}
	return false
}

// ChangeSkill submits one reviewed enable/disable request. The CLI owns the
// API idempotency key and all remote credentials; desktop receives a bounded
// public receipt only.
func ChangeSkill(ctx context.Context, client cliadapter.Client, input SkillChangeInput) (SkillChangeReceipt, error) {
	input = copySkillChangeInput(input)
	if !input.Confirmed {
		return SkillChangeReceipt{}, &cliadapter.OperationError{Code: "confirmation_required"}
	}
	if filepath.Clean(input.Expected.Home) != filepath.Clean(client.Home) || input.Expected.ServerURL == nil || strings.TrimSpace(*input.Expected.ServerURL) == "" {
		return SkillChangeReceipt{}, fault.CLIContextChanged
	}
	if input.ExpectedGeneration < 0 || input.ExpectedGeneration == int64(^uint64(0)>>1) || !canonicalUUID(input.Skill) || (input.Action != "enable" && input.Action != "disable") || len(input.Scope.Tools) > 32 {
		return SkillChangeReceipt{}, fault.CLIProtocol
	}
	for i, tool := range input.Scope.Tools {
		if !validSkillText(tool, 128) || i > 0 && input.Scope.Tools[i-1] >= tool {
			return SkillChangeReceipt{}, fault.CLIProtocol
		}
	}
	if input.Scope.AccountID != nil && !canonicalUUID(*input.Scope.AccountID) {
		return SkillChangeReceipt{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return SkillChangeReceipt{}, err
	}
	if !SkillsChangeSupported(caps) {
		return SkillChangeReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, SkillsChangeMethod, input, &raw); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return SkillChangeReceipt{}, err
		}
		return SkillChangeReceipt{}, fault.CLIProtocol
	}
	var receipt SkillChangeReceipt
	if err := decodeSkillChangeReceipt(raw, &receipt); err != nil || receipt.OperationID == "" || !receipt.Accepted || receipt.Action != input.Action || receipt.Skill != input.Skill {
		return SkillChangeReceipt{}, fault.CLIProtocol
	}
	wantedGeneration := input.ExpectedGeneration
	if receipt.Changed {
		wantedGeneration++
	}
	if receipt.Generation != wantedGeneration {
		return SkillChangeReceipt{}, fault.CLIProtocol
	}
	if !sameSkillContext(receipt.Context, input.Expected) {
		return SkillChangeReceipt{}, fault.CLIContextChanged
	}
	return receipt, nil
}

func copySkillChangeInput(input SkillChangeInput) SkillChangeInput {
	if input.Expected.ServerURL != nil {
		value := *input.Expected.ServerURL
		input.Expected.ServerURL = &value
	}
	if input.Expected.DeviceID != nil {
		value := *input.Expected.DeviceID
		input.Expected.DeviceID = &value
	}
	input.Scope.Tools = append([]string(nil), input.Scope.Tools...)
	if input.Scope.AccountID != nil {
		value := *input.Scope.AccountID
		input.Scope.AccountID = &value
	}
	return input
}

func decodeSkillChangeReceipt(raw json.RawMessage, receipt *SkillChangeReceipt) error {
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	return decoder.Decode(receipt)
}

func canonicalUUID(value string) bool {
	// Keep the dependency surface small: UUIDs are lowercase canonical strings.
	if len(value) != 36 || value == "00000000-0000-0000-0000-000000000000" {
		return false
	}
	for i, c := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func SkillsListSupported(c cliadapter.Capabilities) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, op := range c.Operations {
		if op.Method == SkillsListMethod {
			return op.Effect == "read" && op.Credential == "user" && op.Confirmation == ""
		}
	}
	return false
}

// ListSkills validates capability and context before accepting a projected
// catalog. It deliberately keeps query scope explicit so a stale account or
// server cannot be represented as an empty list.
func ListSkills(ctx context.Context, client cliadapter.Client, expected cliadapter.ConfigContext, options SkillListOptions) (SkillCatalog, error) {
	if filepath.Clean(expected.Home) != filepath.Clean(client.Home) || expected.ServerURL == nil {
		return SkillCatalog{}, fault.CLIContextChanged
	}
	if options.Tool != "" && options.AccountID != "" || len(options.Tool) > 128 || len(options.AccountID) > 128 {
		return SkillCatalog{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return SkillCatalog{}, err
	}
	if !SkillsListSupported(caps) {
		return SkillCatalog{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	params := map[string]any{
		"effective":      options.Effective,
		"include_system": options.IncludeSystem,
	}
	if options.Tool != "" {
		params["tool"] = options.Tool
	}
	if options.AccountID != "" {
		params["account_id"] = options.AccountID
	}
	var result struct {
		Context    cliadapter.ConfigContext `json:"context"`
		Generation int64                    `json:"generation"`
		Items      []SkillSummary           `json:"items"`
		LocalItems []LocalSkillSummary      `json:"local_items"`
	}
	if err := client.Call(ctx, SkillsListMethod, params, &result); err != nil {
		return SkillCatalog{}, err
	}
	if !sameSkillContext(result.Context, expected) {
		return SkillCatalog{}, fault.CLIContextChanged
	}
	if result.Items == nil || result.LocalItems == nil || result.Generation < 0 || len(result.Items) > 4096 || len(result.LocalItems) > 4096 {
		return SkillCatalog{}, fault.CLIProtocol
	}
	seen := make(map[string]struct{}, len(result.Items)+len(result.LocalItems))
	for _, item := range result.Items {
		if !validSkillText(item.ID, 256) || !validSkillText(item.Name, 512) || !validSkillText(item.DefaultRevisionID, 256) || item.RevisionCount < 0 || item.RevisionCount > 100000 || !validSkillText(item.ProjectDiscovery, 128) {
			return SkillCatalog{}, fault.CLIProtocol
		}
		if _, ok := seen[item.ID]; ok {
			return SkillCatalog{}, fault.CLIProtocol
		}
		seen[item.ID] = struct{}{}
		if item.Effective != nil && (!validSkillText(item.Effective.RevisionID, 256) || item.Effective.ExclusionReason != nil && !validSkillText(*item.Effective.ExclusionReason, 512)) {
			return SkillCatalog{}, fault.CLIProtocol
		}
	}
	for _, item := range result.LocalItems {
		if !validSkillText(item.ID, 256) || !validSkillText(item.Name, 512) || !validSkillText(item.AccountID, 256) || !validSkillText(item.Status, 128) || !validSkillText(item.RevisionID, 256) {
			return SkillCatalog{}, fault.CLIProtocol
		}
		if _, ok := seen[item.ID]; ok {
			return SkillCatalog{}, fault.CLIProtocol
		}
		seen[item.ID] = struct{}{}
	}
	return SkillCatalog{Context: result.Context, Generation: result.Generation, Items: result.Items, LocalItems: result.LocalItems}, nil
}

func sameSkillContext(a, b cliadapter.ConfigContext) bool {
	if filepath.Clean(a.Home) != filepath.Clean(b.Home) || a.ServerURL == nil || b.ServerURL == nil || *a.ServerURL != *b.ServerURL {
		return false
	}
	if a.DeviceID == nil || b.DeviceID == nil {
		return a.DeviceID == nil && b.DeviceID == nil
	}
	return *a.DeviceID == *b.DeviceID
}

func validSkillText(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && !strings.ContainsAny(value, "\x00\r\n")
}
