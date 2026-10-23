package i18n_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
)

func readCatalog(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	messages := map[string]string{}
	for key, value := range raw {
		if strings.HasPrefix(key, "@") {
			continue
		}
		var message string
		if err := json.Unmarshal(value, &message); err != nil {
			t.Fatalf("%s: %s: %v", path, key, err)
		}
		if strings.TrimSpace(message) == "" {
			t.Errorf("%s: empty message %s", path, key)
		}
		messages[key] = message
	}
	return messages
}

func args(t *testing.T, text string) map[string]string {
	t.Helper()
	msg, err := i18n.ParseMessage(text)
	if err != nil {
		t.Fatalf("invalid ICU message %q: %v", text, err)
	}
	result := map[string]string{}
	for _, arg := range msg.Args() {
		result[arg.Name] = arg.Kind
	}
	return result
}

func TestAllShippedCatalogsAreComplete(t *testing.T) {
	dir := filepath.Join(testutil.Root(t), "locales")
	source := readCatalog(t, filepath.Join(dir, "en.json"))
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(source) == 0 || len(files) < 2 {
		t.Fatal("source and translated catalogs are required")
	}
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			translated := readCatalog(t, file)
			for key, value := range source {
				other, ok := translated[key]
				if !ok {
					t.Errorf("missing translation: %s", key)
					continue
				}
				if !reflect.DeepEqual(args(t, value), args(t, other)) {
					t.Errorf("placeholder mismatch: %s", key)
				}
			}
			for key := range translated {
				if _, ok := source[key]; !ok {
					t.Errorf("unknown message: %s", key)
				}
			}
		})
	}
}

func TestPluralFallbackAndArgumentsRemainReadable(t *testing.T) {
	for _, tc := range []struct {
		language string
		count    int
		want     string
	}{
		{"en", 0, "No sessions"}, {"en", 1, "1 session"}, {"en", 2, "2 sessions"},
		{"zh-Hans", 0, "无会话"}, {"zh-Hans", 1, "1 个会话"}, {"zh-CN", 3, "3 个会话"},
		{"fr", 2, "2 sessions"},
	} {
		if got := locales.In(tc.language).SessionCount(tc.count); got != tc.want {
			t.Errorf("%s count %d: %q, want %q", tc.language, tc.count, got, tc.want)
		}
	}
	name := "用户的项目 {name}"
	for _, tag := range []string{"en", "zh-Hans", "en-XA"} {
		if got := locales.In(tag).TabTerminals(name, 2); !strings.Contains(got, name) || strings.Contains(got, "{count}") {
			t.Errorf("argument changed or left unformatted for %s: %q", tag, got)
		}
	}
	if got := locales.In("en-XA").OpenProject(); got == locales.In("en").OpenProject() || !strings.HasPrefix(got, "[") {
		t.Fatalf("pseudo locale not applied: %q", got)
	}
}
