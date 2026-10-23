package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/installer"
	"github.com/Agent-Remote/agent-remote-desktop/internal/platform"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo"
)

const developmentVersion = "0.1.0-dev"

func applicationVersion() string {
	if version := strings.TrimSpace(mygo.App.Version()); version != "" {
		return version
	}
	// Plain `go run` has no mygo package metadata. Keep the local developer
	// probe useful; packaged builds always receive mygo.json's version.
	return developmentVersion
}

func main() {
	homeFlag := flag.String("home", "", locales.HomeFlagHelp())
	version := flag.Bool("version", false, locales.VersionFlagHelp())
	flag.Parse()
	if *version {
		// mygo injects the release version from mygo.json into packaged
		// binaries. Read that value at runtime so the CLI probe, installer
		// smoke and OS metadata cannot drift from the artifact manifest.
		fmt.Println("agent-remote-desktop", applicationVersion())
		return
	}
	home, err := platform.Home(*homeFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, presentation.Error(err))
		os.Exit(1)
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, presentation.Error(err))
		os.Exit(1)
	}
	runningLock, err := installer.AcquireRunningLockForExecutable(executable)
	if err != nil {
		fmt.Fprintln(os.Stderr, presentation.Error(err))
		os.Exit(1)
	}
	if runningLock != nil {
		defer func() { _ = runningLock.Close() }()
	}
	a, err := app.New(home)
	if err != nil {
		fmt.Fprintln(os.Stderr, presentation.Error(err))
		os.Exit(1)
	}
	a.Configure()
	err = mygo.App.Run()
	closeErr := a.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, presentation.Error(err))
		os.Exit(1)
	}
	if closeErr != nil {
		fmt.Fprintln(os.Stderr, presentation.Error(closeErr))
		os.Exit(1)
	}
}
