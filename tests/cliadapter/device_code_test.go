package cliadapter_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func TestDeviceCodeProgressArrivesBeforeCompletionAndCancellationDoesNotRetry(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "device-wait")
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sawProgress := false
	_, err := remote.DeviceCodeLogin(ctx, client, remote.DeviceCodeLoginInput{ServerURL: "https://test.invalid", Confirmed: true, SkipDevice: true}, func(event remote.DeviceCodeApproval) {
		sawProgress = event.UserCode == "ABCD-1234" && event.VerificationURL == "https://auth.test.invalid/cli?code=ABCD-1234"
		cancel()
	})
	if !sawProgress || !errors.Is(err, fault.LoginOutcomeUnknown) {
		t.Fatalf("progress/cancellation mismatch: %v %v", sawProgress, err)
	}
	calls, _ := os.ReadFile(log)
	if string(calls) != "login.device_code\n" {
		t.Fatalf("unexpected retry: %q", calls)
	}
}

func TestDeviceCodeStreamRejectsUnsafeAndUncorrelatedEvents(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	for _, mode := range []string{"device-wrong-id", "device-invalid-url", "device-secret-progress", "device-invalid-code", "device-invalid-query", "device-oversized", "device-duplicate-progress", "device-wrong-operation", "device-trailing"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			count := 0
			_, err := remote.DeviceCodeLogin(context.Background(), client, remote.DeviceCodeLoginInput{ServerURL: "https://test.invalid", Confirmed: true, SkipDevice: true}, func(remote.DeviceCodeApproval) { count++ })
			if !errors.Is(err, fault.LoginOutcomeUnknown) {
				t.Fatalf("accepted invalid response: %v", err)
			}
			if count > 1 {
				t.Fatal("published duplicate progress")
			}
			if (strings.Contains(mode, "invalid") || mode == "device-secret-progress" || mode == "device-wrong-id" || mode == "device-oversized") && count != 0 {
				t.Fatal("unsafe approval reached UI")
			}
		})
	}
}

func TestDeviceCodeTerminalReceiptAndExpiryAreDistinct(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	for _, mode := range []string{"device-success", "device-expired"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			result, err := remote.DeviceCodeLogin(context.Background(), client, remote.DeviceCodeLoginInput{ServerURL: "https://test.invalid", Confirmed: true, SkipDevice: true}, nil)
			if mode == "device-success" {
				if err != nil || result.OperationID == "" || result.ServerURL != "https://test.invalid" {
					t.Fatalf("receipt: %+v %v", result, err)
				}
			} else {
				var operation *cliadapter.OperationError
				if !errors.As(err, &operation) || operation.Code != "login_expired" || operation.Retryable {
					t.Fatalf("expiry: %v", err)
				}
			}
		})
	}
}

func TestDeviceCodeInheritedStdoutHasBoundedCancellationAndTerminalDrain(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	for _, cancelOnProgress := range []bool{true, false} {
		t.Run(strconv.FormatBool(cancelOnProgress), func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "device-inherited-stdout")
			pidPath := filepath.Join(t.TempDir(), "pid")
			t.Setenv("DESKTOP_TEST_CHILD_PID", pidPath)
			t.Cleanup(func() {
				data, err := os.ReadFile(pidPath)
				if err != nil {
					t.Error(err)
					return
				}
				pid, err := strconv.Atoi(string(data))
				if err != nil {
					t.Error(err)
					return
				}
				if process, err := os.FindProcess(pid); err == nil {
					_ = process.Kill()
					_ = process.Release()
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			started := time.Now()
			_, err := remote.DeviceCodeLogin(ctx, client, remote.DeviceCodeLoginInput{ServerURL: "https://test.invalid", Confirmed: true, SkipDevice: true}, func(remote.DeviceCodeApproval) {
				if cancelOnProgress {
					cancel()
				}
			})
			if !errors.Is(err, fault.LoginOutcomeUnknown) {
				t.Fatalf("unexpected receipt: %v", err)
			}
			if elapsed := time.Since(started); elapsed > 5*time.Second {
				t.Fatalf("inherited stdout delayed local completion: %v", elapsed)
			}
		})
	}
}

func TestDeviceCodeClosedStdoutDoesNotWaitForLongLivedCLI(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	for _, mode := range []string{"device-early-eof", "device-terminal-eof"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			start := time.Now()
			_, err := remote.DeviceCodeLogin(ctx, client, remote.DeviceCodeLoginInput{ServerURL: "https://test.invalid", Confirmed: true, SkipDevice: true}, nil)
			if !errors.Is(err, fault.LoginOutcomeUnknown) {
				t.Fatalf("invalid stream receipt: %v", err)
			}
			if elapsed := time.Since(start); elapsed > 5*time.Second {
				t.Fatalf("closed stdout delayed completion: %v", elapsed)
			}
		})
	}
}
