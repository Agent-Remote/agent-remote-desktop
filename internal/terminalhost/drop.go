package terminalhost

import (
	"errors"
	"runtime"
	"strings"
	"unicode/utf8"
)

var ErrInvalidDroppedPath = errors.New("invalid dropped path")

const maxDroppedPaths = 32
const maxDroppedPathBytes = 16 * 1024

// FormatDroppedPaths returns shell input suitable for inserting into an
// interactive terminal. It only formats the path as data; it never executes
// it. POSIX paths use single-quote escaping, which keeps whitespace,
// newlines, substitutions and shell operators literal. Windows uses a
// quoted command prompt argument and rejects expansion characters that cannot
// be made literal across cmd.exe and PowerShell.
func FormatDroppedPaths(paths []string, windows bool) (string, error) {
	if len(paths) == 0 || len(paths) > maxDroppedPaths {
		return "", ErrInvalidDroppedPath
	}
	var b strings.Builder
	for i, path := range paths {
		if err := validateDroppedPath(path, windows); err != nil {
			return "", err
		}
		if i > 0 {
			b.WriteByte(' ')
		}
		if windows {
			b.WriteString(quoteWindowsPath(path))
		} else {
			b.WriteString(quotePOSIXPath(path))
		}
	}
	return b.String(), nil
}

func validateDroppedPath(path string, windows bool) error {
	if path == "" || len(path) > maxDroppedPathBytes || !utf8.ValidString(path) || strings.IndexByte(path, 0) >= 0 {
		return ErrInvalidDroppedPath
	}
	if windows {
		// mygo reports native absolute paths. Do not accept relative input,
		// which could otherwise be interpreted relative to a different shell
		// working directory after a hidden-session restore.
		if !(len(path) >= 3 && ((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) && path[1] == ':' && (path[2] == '\\' || path[2] == '/')) && !strings.HasPrefix(path, `\\`) {
			return ErrInvalidDroppedPath
		}
		// cmd.exe expands these even inside double quotes. Rejecting the path
		// is safer than sending an ambiguous token to an Agent shell.
		if strings.ContainsAny(path, "%!") {
			return ErrInvalidDroppedPath
		}
		return nil
	}
	// Keep the explicit POSIX mode deterministic on every host so tests and
	// protocol callers can validate both quoting rules during cross-builds.
	if !strings.HasPrefix(path, "/") {
		return ErrInvalidDroppedPath
	}
	return nil
}

func quotePOSIXPath(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
}

func quoteWindowsPath(path string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range path {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '^':
			b.WriteString("^^")
		case '&', '|', '<', '>', '(', ')':
			b.WriteByte('^')
			b.WriteRune(r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// InsertDroppedPaths sends shell-quoted paths as if the user typed them.
// The terminal plugin owns input ordering and preserves bracketed paste and
// IME state; this helper does not run a command or append Enter.
func (h *Host) InsertDroppedPaths(paths []string) error {
	if h == nil || h.Terminal == nil {
		return ErrInvalidDroppedPath
	}
	select {
	case <-h.done:
		return ErrInvalidDroppedPath
	default:
	}
	text, err := FormatDroppedPaths(paths, runtime.GOOS == "windows")
	if err != nil {
		return err
	}
	h.Terminal.Send([]byte(text))
	return nil
}
