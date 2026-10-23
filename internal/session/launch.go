// Package session manages Agent processes independently of their visible panes.
package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/attachment"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/platform"
)

// ParseArguments accepts only a JSON array of strings. JSON null must not turn
// into an empty argument, and arguments are always passed directly as argv.
func ParseArguments(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if value[0] != '[' {
		return nil, fault.InvalidArguments
	}
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(value), &raw); err != nil {
		return nil, fault.InvalidArguments
	}
	result := make([]string, len(raw))
	for i, item := range raw {
		if len(item) == 0 || item[0] != '"' {
			return nil, fault.InvalidArguments
		}
		if err := json.Unmarshal(item, &result[i]); err != nil {
			return nil, fault.InvalidArguments
		}
		if strings.ContainsRune(result[i], 0) {
			return nil, fault.ArgumentNUL
		}
	}
	return result, nil
}

type Tool string

const (
	Codex          Tool = "codex"
	Claude         Tool = "claude"
	RemoteClaude   Tool = "fclaude"
	AgentRemote    Tool = "agent-remote"
	AccountBinding Tool = "account-binding"
)

func (t Tool) Executable() string {
	if t == AccountBinding {
		return "agent-remote"
	}
	return string(t)
}

func (t Tool) IsRemote() bool { return t == RemoteClaude || t == AccountBinding }

type Launch struct {
	Tool      Tool
	Directory string
	Home      string
	Arguments []string
	// AttachmentTransfer is enabled only after the desktop has observed the
	// exact CLI v1 declaration alongside sessions.attach v1. It does not grant
	// remote authorization; fclaude still performs Server/Node admission.
	AttachmentTransfer bool
	AttachmentLimits   attachment.Limits
}

type Command struct {
	Argv      []string
	Directory string
	Env       []string
	Remote    bool
	// AttachmentTransfer keeps the exact remote attachment transport decision
	// with the frozen launch command. It is ignored for local sessions and is
	// never inferred by terminalhost from mutable UI state.
	AttachmentTransfer bool
	AttachmentLimits   attachment.Limits
}

// RecoveryArguments opens a tool's own picker when no local reference is known.
// Remote history must first refresh and review its full context through the
// remote list. A bare reference cannot authorize recovery on the current server.
func RecoveryArguments(tool Tool, reference string) ([]string, error) {
	reference = strings.TrimSpace(reference)
	if strings.HasPrefix(reference, "-") || strings.ContainsAny(reference, "\x00\r\n") {
		return nil, fault.ResumeReference
	}
	switch tool {
	case Codex:
		if reference == "" {
			return []string{"resume"}, nil
		}
		return []string{"resume", reference}, nil
	case Claude:
		if reference == "" {
			return []string{"--resume"}, nil
		}
		return []string{"--resume", reference}, nil
	case RemoteClaude:
		return nil, fault.ResumeReference
	default:
		return nil, fault.UnsupportedAgent
	}
}

func Prepare(request Launch) (Command, error) {
	return PrepareWithResolver(request, platform.Executable)
}

// PrepareWithResolver supports configured or portable executable locations.
// Resolution happens at launch time rather than trusting cached UI detection.
func PrepareWithResolver(request Launch, findExecutable func(string) (string, error)) (Command, error) {
	if findExecutable == nil {
		findExecutable = platform.Executable
	}
	if request.Tool != Codex && request.Tool != Claude && request.Tool != RemoteClaude && request.Tool != AgentRemote && request.Tool != AccountBinding {
		return Command{}, fault.UnsupportedAgent
	}
	if request.Tool == AccountBinding && !validBindingArguments(request.Arguments) {
		return Command{}, fault.InvalidArguments
	}
	dir, err := filepath.Abs(request.Directory)
	if err != nil {
		return Command{}, err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return Command{}, err
	}
	if !info.IsDir() {
		return Command{}, fault.ProjectDirectory
	}
	binary, err := findExecutable(request.Tool.Executable())
	if err != nil {
		return Command{}, err
	}
	for _, arg := range request.Arguments {
		if strings.ContainsRune(arg, 0) {
			return Command{}, fault.ArgumentNUL
		}
	}
	env := []string{"AGENT_REMOTE_HOME=" + request.Home}
	// A GUI-launched Node-based Agent needs the executable's installation directory.
	env = append(env, "PATH="+filepath.Dir(binary)+string(os.PathListSeparator)+os.Getenv("PATH"))
	return Command{Argv: append([]string{binary}, request.Arguments...), Directory: dir, Env: env, Remote: request.Tool.IsRemote(), AttachmentTransfer: request.AttachmentTransfer, AttachmentLimits: request.AttachmentLimits}, nil
}
