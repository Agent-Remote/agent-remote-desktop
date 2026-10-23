// command-catalog exports syntax from the actual CLI binaries for coverage review.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
)

func main() {
	cli := flag.String("cli", "agent-remote", locales.CliBinaryFlagHelp())
	launcher := flag.String("launcher", "fclaude", locales.LauncherBinaryFlagHelp())
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var schemas []cliadapter.Schema
	for _, entry := range []struct{ path, name string }{{*cli, "agent-remote"}, {*launcher, "fclaude"}} {
		s, err := cliadapter.Discover(ctx, entry.path, entry.name)
		if err != nil {
			fmt.Fprintln(os.Stderr, presentation.Error(err))
			os.Exit(1)
		}
		schemas = append(schemas, s)
	}
	e := json.NewEncoder(os.Stdout)
	e.SetIndent("", "  ")
	if err := e.Encode(schemas); err != nil {
		fmt.Fprintln(os.Stderr, presentation.Error(err))
		os.Exit(1)
	}
}
