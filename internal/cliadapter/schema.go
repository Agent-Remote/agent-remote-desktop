// Package cliadapter owns the versioned interface to the existing Rust client.
package cliadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const SchemaVersion = 1
const maxSchemaBytes = 4 * 1024 * 1024

type Schema struct {
	Version    int       `json:"schema_version"`
	Executable string    `json:"executable"`
	CLIVersion string    `json:"cli_version"`
	Commands   []Command `json:"commands"`
}
type Command struct {
	Path           []string   `json:"path"`
	Description    string     `json:"description"`
	Arguments      []Argument `json:"arguments"`
	HasSubcommands bool       `json:"has_subcommands"`
	Platforms      []string   `json:"platforms,omitempty"`
}
type Argument struct {
	ID          string   `json:"id"`
	Long        string   `json:"long"`
	Short       string   `json:"short"`
	Position    *int     `json:"positional_index"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Global      bool     `json:"global"`
	Action      string   `json:"action"`
	MinValues   int      `json:"min_values"`
	MaxValues   *int     `json:"max_values"`
	ValueNames  []string `json:"value_names"`
	Choices     []string `json:"choices"`
	Defaults    []string `json:"defaults"`
	ValueHint   string   `json:"value_hint"`
}

// Discover never falls back to scraping human help or running a mutation.
func Discover(ctx context.Context, binary, expected string) (Schema, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "--desktop-schema")
	cmd.WaitDelay = time.Second
	out := boundedBuffer{limit: maxSchemaBytes}
	out.onLimit = func() { _ = cmd.Process.Kill() }
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return Schema{}, ctx.Err()
		}
		if out.exceeded {
			return Schema{}, fault.CLISchema
		}
		return Schema{}, fmt.Errorf("%w: %w", fault.CLIUnavailable, err)
	}
	return Decode(out.Bytes(), expected)
}

func Decode(data []byte, expected string) (Schema, error) {
	if len(data) > maxSchemaBytes {
		return Schema{}, fault.CLISchema
	}
	var schema Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return schema, fault.CLISchema
	}
	if schema.Version != SchemaVersion {
		return Schema{}, fmt.Errorf("%w: version %d", fault.CLISchema, schema.Version)
	}
	if schema.Executable != expected || schema.CLIVersion == "" || len(schema.Commands) == 0 {
		return Schema{}, fault.CLISchema
	}
	if len(schema.Commands) > 1024 {
		return Schema{}, fault.CLISchema
	}
	seen := make(map[string]bool)
	for _, command := range schema.Commands {
		key := strings.Join(command.Path, " ")
		if seen[key] {
			return Schema{}, fault.CLISchema
		}
		seen[key] = true
		for _, part := range command.Path {
			if part == "" || strings.ContainsAny(part, " \t\r\n\x00") {
				return Schema{}, fault.CLISchema
			}
		}
		platforms := make(map[string]bool)
		if command.Platforms != nil && len(command.Platforms) == 0 {
			return Schema{}, fault.CLISchema
		}
		for _, platform := range command.Platforms {
			if platforms[platform] || (platform != "macos" && platform != "windows" && platform != "linux") {
				return Schema{}, fault.CLISchema
			}
			platforms[platform] = true
		}
		argumentIDs := make(map[string]bool)
		positionalIndexes := make(map[int]bool)
		for _, argument := range command.Arguments {
			if argument.ID == "" || argumentIDs[argument.ID] || strings.ContainsAny(argument.ID, " \t\r\n\x00") {
				return Schema{}, fault.CLISchema
			}
			argumentIDs[argument.ID] = true
			if argument.MinValues < 0 || argument.MaxValues != nil && *argument.MaxValues < 0 {
				return Schema{}, fault.CLISchema
			}
			if argument.MaxValues != nil && argument.MinValues > *argument.MaxValues {
				return Schema{}, fault.CLISchema
			}
			if argument.Position != nil {
				if *argument.Position < 1 || positionalIndexes[*argument.Position] {
					return Schema{}, fault.CLISchema
				}
				positionalIndexes[*argument.Position] = true
			}
			if strings.ContainsAny(argument.Long, " \t\r\n\x00") || strings.ContainsAny(argument.Short, " \t\r\n\x00") {
				return Schema{}, fault.CLISchema
			}
		}
	}
	return schema, nil
}

type boundedBuffer struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
	onLimit  func()
}

func (b *boundedBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (b *boundedBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.buffer.Len() {
		b.exceeded = true
		if b.onLimit != nil {
			b.onLimit()
		}
		return 0, fault.CLISchema
	}
	return b.buffer.Write(p)
}
