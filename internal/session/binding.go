package session

import (
	"net/url"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

// BindingArguments binds a native connection to the account, login attempt,
// server and device reviewed by the user. It never accepts server command text.
func BindingArguments(server, device, account, binding string) ([]string, error) {
	if !validBindingIdentity(account, binding) || !validDevice(device) {
		return nil, fault.InvalidArguments
	}
	u, err := url.Parse(server)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(server, "\x00\r\n") {
		return nil, fault.InvalidArguments
	}
	return []string{"account", "attach", account, "--binding-session", binding, "--expected-server", server, "--expected-device", device}, nil
}

// BindingReference stores only public identities. Restoring it must refresh the
// account and obtain new confirmation; RecoveryArguments deliberately rejects it.
func BindingReference(account, binding string) (string, error) {
	if !validBindingIdentity(account, binding) {
		return "", fault.ResumeReference
	}
	return account + "/" + binding, nil
}

func BindingAccount(reference string) (string, error) {
	account, binding, ok := strings.Cut(reference, "/")
	if !ok || !validBindingIdentity(account, binding) {
		return "", fault.ResumeReference
	}
	return account, nil
}

func validBindingIdentity(account, binding string) bool {
	if len(account) != 36 {
		return false
	}
	for i := range len(account) {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if account[i] != '-' {
				return false
			}
		} else if !lowerHex(account[i]) {
			return false
		}
	}
	compact := strings.ReplaceAll(account, "-", "")
	prefix := "bind-" + compact[:12] + "-"
	if !strings.HasPrefix(binding, prefix) || len(binding) != len(prefix)+12 {
		return false
	}
	for i := len(prefix); i < len(binding); i++ {
		if !lowerHex(binding[i]) {
			return false
		}
	}
	return true
}

func lowerHex(value byte) bool {
	return (value >= '0' && value <= '9') || (value >= 'a' && value <= 'f')
}

func validDevice(device string) bool {
	if len(device) == 0 || len(device) > 128 || device[0] == '-' {
		return false
	}
	for _, r := range device {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == ':') {
			return false
		}
	}
	return true
}

func validBindingArguments(args []string) bool {
	if len(args) != 9 || args[0] != "account" || args[1] != "attach" || args[3] != "--binding-session" || args[5] != "--expected-server" || args[7] != "--expected-device" {
		return false
	}
	_, err := BindingArguments(args[6], args[8], args[2], args[4])
	return err == nil
}
