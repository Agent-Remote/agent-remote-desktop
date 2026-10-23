package cliadapter_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

// The parent never changes its HOME or default Mutagen environment. A child
// owns one short, temporary namespace, every spawned process, and all cleanup.
func TestMutagenEnvironmentWithBuiltRustCLIAndOfficialDaemon(t *testing.T) {
	cli, mutagen := os.Getenv("DESKTOP_CLI_BINARY"), os.Getenv("DESKTOP_MUTAGEN_BINARY")
	if cli == "" || mutagen == "" {
		t.Skip("set DESKTOP_CLI_BINARY and DESKTOP_MUTAGEN_BINARY for isolated real-daemon interoperability")
	}
	if runtime.GOOS == "windows" {
		t.Skip("this fixture uses Unix executable links; Windows native acceptance is separate")
	}
	if root := os.Getenv("DESKTOP_ENVIRONMENT_INTEROP_CHILD"); root != "" {
		exerciseEnvironmentInterop(t, root, cli, mutagen)
		return
	}
	root, err := os.MkdirTemp("/tmp", "ard-env-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	isolatedHome := filepath.Join(root, "os-home")
	if err := os.Mkdir(isolatedHome, 0700); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestMutagenEnvironmentWithBuiltRustCLIAndOfficialDaemon$", "-test.v")
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key != "HOME" && key != "MUTAGEN_DATA_DIRECTORY" && key != "DESKTOP_ENVIRONMENT_INTEROP_CHILD" {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "HOME="+isolatedHome, "MUTAGEN_DATA_DIRECTORY="+filepath.Join(root, "data"), "DESKTOP_ENVIRONMENT_INTEROP_CHILD="+root)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated interoperability failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "PASS real stopped inventory, exact restart, paused/config preservation, drift rejection and no credentials") {
		t.Fatalf("child did not exercise the contract: %s", output)
	}
	t.Log(string(output))
}

type environmentDiskEntry struct {
	Mode     os.FileMode
	Size     int64
	Modified time.Time
	Digest   string
}

func environmentDisk(t *testing.T, root string) map[string]environmentDiskEntry {
	t.Helper()
	entries := map[string]environmentDiskEntry{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := environmentDiskEntry{Mode: info.Mode(), Size: info.Size(), Modified: info.ModTime()}
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value.Digest = fmt.Sprintf("%x", sha256.Sum256(data))
		}
		key, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entries[key] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return entries
}
func exerciseEnvironmentInterop(t *testing.T, root, cli, mutagen string) {
	home, data := filepath.Join(root, "new-home"), filepath.Join(root, "data")
	bin := filepath.Join(home, "bin")
	if err := os.MkdirAll(bin, 0700); err != nil {
		t.Fatal(err)
	}
	mutagen, err := filepath.Abs(mutagen)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(mutagen, filepath.Join(bin, "mutagen")); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(root, "unused-ssh")
	if err := os.WriteFile(shim, []byte("#!/bin/sh\nexit 77\n"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ssh", "scp"} {
		if err := os.Symlink(shim, filepath.Join(bin, name)); err != nil {
			t.Fatal(err)
		}
	}
	command := func(ctx context.Context, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, filepath.Join(bin, "mutagen"), args...)
		cmd.Env = append(os.Environ(), "MUTAGEN_DATA_DIRECTORY="+data, "MUTAGEN_DISABLE_AUTOSTART=1")
		return cmd
	}
	run := func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return command(ctx, args...).CombinedOutput()
	}
	success := func(args ...string) {
		t.Helper()
		if output, err := run(args...); err != nil {
			t.Fatalf("official Mutagen %v failed: %v %s", args, err, output)
		}
	}
	original := command(context.Background(), "daemon", "run")
	original.Env = append(original.Env, "AGENT_REMOTE_HOME="+filepath.Join(root, "old-home"), "MUTAGEN_SSH_PATH="+filepath.Join(root, "old-bin"))
	if err := original.Start(); err != nil {
		t.Fatal(err)
	}
	originalDone := make(chan error, 1)
	go func() { originalDone <- original.Wait() }()
	t.Cleanup(func() {
		_, _ = run("daemon", "stop")
		select {
		case <-originalDone:
		default:
			_ = original.Process.Kill()
			select {
			case <-originalDone:
			case <-time.After(5 * time.Second):
				t.Error("original daemon was not reaped")
			}
		}
	})
	ready := false
	for deadline := time.Now().Add(6 * time.Second); time.Now().Before(deadline); {
		if _, err := run("sync", "list", "--template", "{{len .}}"); err == nil {
			ready = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !ready {
		t.Fatal("isolated official daemon did not start")
	}
	alpha, beta := filepath.Join(root, "alpha"), filepath.Join(root, "beta")
	for _, path := range []string{alpha, beta} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(alpha, "keep.txt"), []byte("preserve this project file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	success("sync", "create", "--paused", "--no-global-configuration", "--name", "environment-sync", alpha, beta)
	success("forward", "create", "--paused", "--no-global-configuration", "--name", "environment-forward", "tcp:127.0.0.1:0", "tcp:127.0.0.1:9")
	success("daemon", "stop")
	select {
	case err := <-originalDone:
		if err != nil {
			t.Fatal(err)
		}
		originalDone <- nil
	case <-time.After(5 * time.Second):
		t.Fatal("original daemon did not stop")
	}
	before := environmentDisk(t, data)
	client := cliadapter.Client{Binary: cli, Home: home}
	plan, err := remote.PreviewMutagenEnvironment(context.Background(), client)
	if err != nil || !plan.Repairable || plan.DaemonState != "stopped" || len(plan.Sessions) != 2 {
		t.Fatalf("stopped preview %+v %v", plan, err)
	}
	if !plan.Sessions[0].Paused || !plan.Sessions[1].Paused || plan.Sessions[0].Kind == plan.Sessions[1].Kind {
		t.Fatal("complete paused sync+forward scope missing")
	}
	if !reflect.DeepEqual(before, environmentDisk(t, data)) {
		t.Fatal("preview modified stopped daemon data")
	}
	if output, err := run("sync", "list"); err == nil {
		t.Fatalf("preview started the daemon: %s", output)
	}
	for _, name := range []string{"config.toml", "config-write.lock", "state.sqlite3", "secrets"} {
		if _, err := os.Lstat(filepath.Join(home, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("preview created shared state %s: %v", name, err)
		}
	}
	input := remote.MutagenEnvironmentApplyInput{Confirmed: true, ExpectedHome: home, ExpectedPlan: plan}
	receipt, err := remote.ApplyMutagenEnvironment(context.Background(), client, input)
	if err != nil || receipt.Outcome != "completed" || receipt.Environment == nil {
		t.Fatalf("real repair %+v %v", receipt, err)
	}
	if !reflect.DeepEqual(plan.Sessions, receipt.Environment.Sessions) || receipt.Stages[0].State != "skipped" || !reflect.DeepEqual(receipt.Environment.Previous, &plan.Desired) {
		t.Fatal("restart changed paused sessions or failed to record desired environment")
	}
	contents, err := os.ReadFile(filepath.Join(alpha, "keep.txt"))
	if err != nil || string(contents) != "preserve this project file\n" {
		t.Fatal("restart damaged project file")
	}
	_, err = remote.ApplyMutagenEnvironment(context.Background(), client, input)
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) || operation.Code != "sync_mutagen_environment_changed" {
		t.Fatalf("old confirmation reused: %v", err)
	}
	running, err := remote.PreviewMutagenEnvironment(context.Background(), client)
	if err != nil || running.DaemonState != "running" {
		t.Fatalf("running preview %+v %v", running, err)
	}
	success("sync", "resume", "environment-sync")
	_, err = remote.ApplyMutagenEnvironment(context.Background(), client, remote.MutagenEnvironmentApplyInput{Confirmed: true, ExpectedHome: home, ExpectedPlan: running})
	if !errors.As(err, &operation) || operation.Code != "sync_mutagen_environment_changed" {
		t.Fatalf("paused drift accepted: %v", err)
	}
	success("sync", "pause", "environment-sync")
	fresh, err := remote.PreviewMutagenEnvironment(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := remote.ApplyMutagenEnvironment(context.Background(), client, remote.MutagenEnvironmentApplyInput{Confirmed: true, ExpectedHome: home, ExpectedPlan: fresh})
	if err != nil || restarted.Outcome != "completed" || restarted.Environment == nil || restarted.Stages[0].State != "completed" {
		t.Fatalf("running restart %+v %v", restarted, err)
	}
	if reflect.DeepEqual(fresh.RunningFingerprint, restarted.Environment.RunningFingerprint) {
		t.Fatal("restart retained old daemon identity")
	}
	for _, name := range []string{"config.toml", "state.sqlite3", "secrets"} {
		if _, err := os.Lstat(filepath.Join(home, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("local-only operation created authenticated state %s: %v", name, err)
		}
	}
	t.Log("PASS real stopped inventory, exact restart, paused/config preservation, drift rejection and no credentials")
}
