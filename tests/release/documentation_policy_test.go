package release_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

// Release policy and repository-layout rules are user-facing contracts. Keep
// the two READMEs and the operational documents synchronized so a future edit
// cannot silently reintroduce FreeBSD, publisher-signing, or split test-tree
// claims in only one language or document.
func TestReleaseAndDevelopmentDocsDeclareCurrentProjectPolicy(t *testing.T) {
	root := testutil.Root(t)
	documents := map[string][]string{
		"README.md": {
			"macOS uses the project's community self-signing identity",
			"Windows/Linux do not require publisher code signing",
			"FreeBSD is not planned",
			"mygo's official i18n catalogs",
			"repository's root `tests/`",
		},
		"README.zh-CN.md": {
			"macOS 沿用项目社区自签",
			"Windows/Linux 不要求发行者代码签名",
			"FreeBSD 不在计划中",
			"mygo 官方 i18n",
			"根目录 `tests/`",
		},
		"docs/release-build.md": {
			"macOS uses the project's persistent self-signed",
			"Windows and Linux do not require publisher code signing",
			"FreeBSD is not a release target",
			"Apple Developer ID and notarization",
		},
		"docs/platform-validation.md": {
			"正式版目标为 Windows、macOS、Linux，FreeBSD 不在计划中",
			"macOS 社区自签",
			"Windows/Linux 不要求发行者代码签名",
			"根 `tests/`",
		},
		"docs/development-standards.md": {
			"github.com/egoist/mygo/i18n",
			"agent-remote-desktop/tests/",
			"任何自有测试文件均不得与其混放",
		},
	}

	for relative, markers := range documents {
		t.Run(relative, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(root, relative))
			if err != nil {
				t.Fatalf("required project document is missing: %v", err)
			}
			// Treat line wrapping as presentation: policy assertions should remain
			// stable when a maintainer reflows a paragraph.
			content := strings.Join(strings.Fields(string(data)), " ")
			for _, marker := range markers {
				if !strings.Contains(content, strings.Join(strings.Fields(marker), " ")) {
					t.Errorf("%s no longer states required policy marker %q", relative, marker)
				}
			}
		})
	}
}
