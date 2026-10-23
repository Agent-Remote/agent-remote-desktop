package cliadapter_test

import (
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
)

func TestDiscoveryRejectsWrongProtocolOrExecutable(t *testing.T) {
	for _, raw := range []string{
		`{"schema_version":2,"executable":"agent-remote","cli_version":"1","commands":[{}]}`,
		`{"schema_version":1,"executable":"fclaude","cli_version":"1","commands":[{}]}`,
		`{"schema_version":1,"executable":"agent-remote","cli_version":"1","commands":[]}`,
		`human output`,
	} {
		if _, err := cliadapter.Decode([]byte(raw), "agent-remote"); err == nil {
			t.Fatalf("accepted incompatible schema: %s", raw)
		}
	}
}

func TestDiscoveryPreservesUnboundedPositionalArgument(t *testing.T) {
	raw := `{"schema_version":1,"executable":"fclaude","cli_version":"0.2.58","commands":[{"path":["new"],"arguments":[{"id":"claude_args","positional_index":1,"action":"Append","max_values":null}]}]}`
	s, err := cliadapter.Decode([]byte(raw), "fclaude")
	if err != nil {
		t.Fatal(err)
	}
	a := s.Commands[0].Arguments[0]
	if a.Position == nil || *a.Position != 1 || a.MaxValues != nil || a.Action != "Append" {
		t.Fatalf("lost argument semantics: %+v", a)
	}
}

func TestSchemaPlatformMetadataCannotSilentlyBroadenSupport(t *testing.T) {
	for _, platforms := range []string{`[]`, `["freebsd"]`, `["macos","macos"]`, `"macos"`} {
		raw := `{"schema_version":1,"executable":"agent-remote","cli_version":"1","commands":[{"path":["device","install"],"platforms":` + platforms + `}]}`
		if _, err := cliadapter.Decode([]byte(raw), "agent-remote"); err == nil {
			t.Fatalf("accepted invalid platform metadata: %s", platforms)
		}
	}
	for _, raw := range []string{
		`{"schema_version":1,"executable":"agent-remote","cli_version":"1","commands":[{"path":["device"]},{"path":["device"]}]}`,
		`{"schema_version":1,"executable":"agent-remote","cli_version":"1","commands":[{"path":["device install"]}]}`,
	} {
		if _, err := cliadapter.Decode([]byte(raw), "agent-remote"); err == nil {
			t.Fatal("accepted ambiguous command path")
		}
	}
}

func TestSchemaRejectsAmbiguousArgumentDefinitions(t *testing.T) {
	cases := []string{
		`{"id":"value","min_values":1,"max_values":1},{"id":"value","min_values":0,"max_values":1}`,
		`{"id":"first","positional_index":1},{"id":"second","positional_index":1}`,
		`{"id":"value","min_values":2,"max_values":1}`,
		`{"id":"bad id"}`,
	}
	for _, arguments := range cases {
		raw := `{"schema_version":1,"executable":"agent-remote","cli_version":"1","commands":[{"path":["status"],"arguments":[` + arguments + `]}]}`
		if _, err := cliadapter.Decode([]byte(raw), "agent-remote"); err == nil {
			t.Fatalf("accepted ambiguous argument definition: %s", arguments)
		}
	}
}
