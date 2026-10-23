package remote

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
)

func validDefaultID(s *string) bool { return s == nil || validDefaultTool(*s) }

// validCanonicalUUID mirrors the Rust desktop-control boundary. Exact
// session operations use UUIDs as stable caller-key and runtime identities;
// accepting a slug here would only defer a malformed request to the CLI (or,
// worse, make a receipt look like it belongs to a different session). The
// comparison is deliberately textual so uppercase, braces and alternate UUID
// spellings are rejected just as uuid::Uuid::to_string() is in the CLI.
func validCanonicalUUID(value string) bool {
	if len(value) != 36 || value == "00000000-0000-0000-0000-000000000000" {
		return false
	}
	for index := 0; index < len(value); index++ {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if value[index] != '-' {
				return false
			}
			continue
		}
		ch := value[index]
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f')) {
			return false
		}
	}
	return true
}

func validDefaultTool(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}
func validDefaultContext(c cliadapter.ConfigContext) bool {
	if !validWireGuardPath(c.Home) || (c.DeviceID != nil && !validDefaultTool(*c.DeviceID)) {
		return false
	}
	if c.ServerURL == nil {
		return true
	}
	return *c.ServerURL != "" && len(*c.ServerURL) <= 8192 && strings.IndexFunc(*c.ServerURL, unicode.IsControl) < 0
}

// Reject unknown, missing, duplicate and null scalar fields. Required nullable
// values remain distinguishable from omitted values in both result and context.
func strictDefaultJSON(raw json.RawMessage, value any, keys ...string) error {
	if err := uniqueDefaultJSON(raw); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != len(keys) {
		return fmt.Errorf("default shape")
	}
	for _, key := range keys {
		field, exists := fields[key]
		if !exists || (bytes.Equal(field, []byte("null")) && key != "account_id" && key != "previous_account_id" && key != "server_url" && key != "active_device_id") {
			return fmt.Errorf("default field")
		}
	}
	if context, ok := fields["context"]; ok {
		var c cliadapter.ConfigContext
		if err := strictDefaultJSON(context, &c, "home", "server_url", "active_device_id"); err != nil {
			return err
		}
	}
	return nil
}

func uniqueDefaultJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	var read func() error
	read = func() error {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		switch delimiter {
		case '{':
			seen := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				s, ok := key.(string)
				if !ok || seen[s] {
					return fmt.Errorf("duplicate field")
				}
				seen[s] = true
				if err := read(); err != nil {
					return err
				}
			}
		case '[':
			for decoder.More() {
				if err := read(); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("invalid delimiter")
		}
		_, err = decoder.Token()
		return err
	}
	if err := read(); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("trailing value")
	}
	return nil
}
