package remote

import (
	"encoding/json"
	"slices"
	"strings"
	"unicode"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

func validAccountImportSelection(selection AccountImportSelection) bool {
	return selection.Tool == "claude"
}
func decodeAccountImportSelection(raw json.RawMessage) (AccountImportSelection, error) {
	var value AccountImportSelection
	if _, err := decodeAccountObject(raw, &value, []string{"tool", "include_resume_history", "exclude_skills"}, nil); err != nil || !validAccountImportSelection(value) {
		return value, fault.CLIProtocol
	}
	return value, nil
}
func validAccountImportPath(value string) bool {
	if len(value) > 1024 || strings.Contains(value, "\\") || strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return false
	}
	if value == "~/.claude.json" {
		return true
	}
	if !strings.HasPrefix(value, "~/.claude/") {
		return false
	}
	for _, part := range strings.Split(strings.TrimPrefix(value, "~/"), "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}
func validAccountImportPaths(paths []string) bool {
	if paths == nil || len(paths) > 2048 {
		return false
	}
	seen := map[string]bool{}
	for _, value := range paths {
		if !validAccountImportPath(value) || seen[value] {
			return false
		}
		seen[value] = true
	}
	return true
}
func validAccountImportScope(selection AccountImportSelection, include, exclude []string) bool {
	if len(include) > 64 || !validAccountImportPaths(include) || !validAccountImportPaths(exclude) {
		return false
	}
	expected := []string{"~/.claude.json", "~/.claude/cache", "~/.claude/logs", "~/.claude/transcripts"}
	if selection.ExcludeSkills {
		expected = append(expected, "~/.claude/skills")
	}
	if len(exclude) != len(expected) {
		return false
	}
	for _, path := range expected {
		if !slices.Contains(exclude, path) {
			return false
		}
	}
	for _, path := range include {
		name := strings.TrimPrefix(path, "~/.claude/")
		if strings.Contains(name, "/") || slices.Contains(exclude, path) {
			return false
		}
		normal := slices.Contains([]string{"settings.json", "CLAUDE.md", "agents", "skills", "plugins", "hooks", "rules"}, name) || strings.HasSuffix(name, ".md")
		history := selection.IncludeResumeHistory && slices.Contains([]string{"projects", "sessions", "history.jsonl", "file-history", "plans", "tasks", "session-env"}, name)
		if !normal && !history {
			return false
		}
	}
	return true
}
func importPathUnder(path string, roots []string) bool {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}
func decodeAccountImportDiscovery(raw json.RawMessage) (AccountImportDiscovery, error) {
	var value AccountImportDiscovery
	fields, err := decodeAccountObject(raw, &value, []string{"source_home", "selection", "include", "exclude"}, nil)
	if err != nil || !validWireGuardPath(value.SourceHome) || !validAccountImportPaths(value.Include) || !validAccountImportPaths(value.Exclude) {
		return value, fault.CLIProtocol
	}
	value.Selection, err = decodeAccountImportSelection(fields["selection"])
	if err == nil && !validAccountImportScope(value.Selection, value.Include, value.Exclude) {
		err = fault.CLIProtocol
	}
	return value, err
}
func decodeAccountImportPlan(raw json.RawMessage) (AccountImportPlan, error) {
	var plan AccountImportPlan
	fields, err := decodeAccountObject(raw, &plan, []string{"context", "account_id", "selection", "source_home", "include", "exclude", "manifest", "total_bytes", "source_fingerprint", "intent_fingerprint", "digest"}, nil)
	if err != nil || !validDefaultTool(plan.AccountID) || !validWireGuardPath(plan.SourceHome) || !validAccountImportPaths(plan.Include) || !validAccountImportPaths(plan.Exclude) || !validSyncFingerprint(plan.SourceFingerprint) || !validSyncFingerprint(plan.IntentFingerprint) || !validSyncFingerprint(plan.Digest) || plan.TotalBytes > 8*1024*1024 {
		return plan, fault.CLIProtocol
	}
	if plan.Context, err = decodeAccountContext(fields["context"]); err != nil {
		return plan, err
	}
	if plan.Selection, err = decodeAccountImportSelection(fields["selection"]); err != nil {
		return plan, err
	}
	if !validAccountImportScope(plan.Selection, plan.Include, plan.Exclude) {
		return plan, fault.CLIProtocol
	}
	var files []json.RawMessage
	if json.Unmarshal(fields["manifest"], &files) != nil || files == nil || len(files) > 2048 {
		return plan, fault.CLIProtocol
	}
	seen := map[string]bool{}
	var total uint64
	for _, rawFile := range files {
		var file AccountImportFile
		if _, err := decodeAccountObject(rawFile, &file, []string{"path", "size", "sha256", "mode"}, nil); err != nil || !validAccountImportPath(file.Path) || seen[file.Path] || !importPathUnder(file.Path, plan.Include) || importPathUnder(file.Path, plan.Exclude) || file.Size > 1024*1024 || file.Mode != 384 || !validSyncFingerprint(file.SHA256) {
			return plan, fault.CLIProtocol
		}
		seen[file.Path] = true
		total += file.Size
	}
	if total != plan.TotalBytes {
		return plan, fault.CLIProtocol
	}
	return plan, nil
}
