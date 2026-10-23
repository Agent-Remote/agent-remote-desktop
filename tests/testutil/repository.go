// Package testutil contains helpers shared only by repository tests.
package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

func Root(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "mygo.json")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("desktop repository root not found")
		}
		dir = parent
	}
}
