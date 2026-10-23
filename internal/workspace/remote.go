package workspace

import (
	"net/url"
	"path/filepath"
	"strings"
	"unicode"
)

// RemoteContext is a public, exact terminal destination. It contains no
// credentials and never proves that a remote or local process survived.
type RemoteContext struct {
	Home      string `json:"home"`
	ServerURL string `json:"server_url"`
	DeviceID  string `json:"device_id"`
	SessionID string `json:"session_id"`
}

// Valid reports whether a persisted runtime identity contains only bounded,
// public metadata. The identity is an observation and is never used to
// authorize a reconnect; the current remote inventory must still be queried.
func (r RemoteSessionInfo) Valid() bool {
	for _, value := range []string{r.AccountID, r.NodeID, r.Backend} {
		if value == "" || len(value) > 256 || strings.IndexFunc(value, func(c rune) bool { return unicode.IsSpace(c) || unicode.IsControl(c) }) >= 0 {
			return false
		}
	}
	return true
}

func (r RemoteContext) Valid() bool {
	if !filepath.IsAbs(r.Home) || strings.ContainsAny(r.Home, "\x00\r\n") || !canonicalUUID(r.DeviceID) || !canonicalUUID(r.SessionID) {
		return false
	}
	u, err := url.Parse(r.ServerURL)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && !u.ForceQuery && u.Fragment == "" && len(r.ServerURL) <= 4096 && strings.IndexFunc(r.ServerURL, func(c rune) bool { return unicode.IsSpace(c) || unicode.IsControl(c) }) < 0
}

func (r *RemoteContext) Clone() *RemoteContext {
	if r == nil {
		return nil
	}
	copy := *r
	return &copy
}

func canonicalUUID(value string) bool {
	if len(value) != 36 || value == "00000000-0000-0000-0000-000000000000" {
		return false
	}
	for i := range len(value) {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if value[i] != '-' {
				return false
			}
		} else if !((value[i] >= '0' && value[i] <= '9') || (value[i] >= 'a' && value[i] <= 'f')) {
			return false
		}
	}
	return true
}
