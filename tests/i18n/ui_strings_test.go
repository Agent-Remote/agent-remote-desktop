package i18n_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"testing"
	"unicode"

	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

// This gate catches direct UI literals. Reviews still check composed messages and
// data-driven labels, which cannot be classified reliably by a syntax check.
func TestUIStringsUseCatalogs(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(testutil.Root(t), "internal", "app", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	// Brand text and provider examples are catalog messages too; keeping this
	// allowlist empty makes new desktop-owned copy fail the gate immediately.
	allowed := map[string]bool{}
	for _, file := range files {
		set := token.NewFileSet()
		tree, err := parser.ParseFile(set, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		check := func(expr ast.Expr) {
			lit, ok := expr.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatal(err)
			}
			if allowed[value] {
				return
			}
			for _, r := range value {
				if unicode.IsLetter(r) {
					t.Errorf("%s: user-facing literal must use locales: %q", set.Position(lit.Pos()), value)
					break
				}
			}
		}
		ast.Inspect(tree, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.CallExpr:
				selector, ok := n.Fun.(*ast.SelectorExpr)
				if !ok {
					break
				}
				if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "ui" {
					switch selector.Sel.Name {
					case "Text", "Button", "PrimaryButton":
						if len(n.Args) > 1 {
							check(n.Args[1])
						}
					}
				} else {
					switch selector.Sel.Name {
					case "Label", "Tooltip", "Placeholder", "SetTitle", "SetToolTip":
						if len(n.Args) > 0 {
							check(n.Args[0])
						}
					}
				}
			case *ast.KeyValueExpr:
				if key, ok := n.Key.(*ast.Ident); ok {
					switch key.Name {
					case "Label", "Title", "ToolTip":
						check(n.Value)
					}
				}
			}
			return true
		})
	}
}
