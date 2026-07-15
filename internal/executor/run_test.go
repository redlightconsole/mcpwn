package executor

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"mcpwn/internal/config"
)

func TestExecuteRunsCommand(t *testing.T) {
	withHelperProcess(t)

	result, err := Execute(context.Background(), &config.Tool{Command: os.Args[0]}, []string{
		"-test.run=TestHelperProcess",
		"--",
		"success",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Output != "ok\n" {
		t.Fatalf("Execute() output = %q, want %q", result.Output, "ok\n")
	}
}

func TestExecuteReturnsExitError(t *testing.T) {
	withHelperProcess(t)

	result, err := Execute(context.Background(), &config.Tool{Command: os.Args[0]}, []string{
		"-test.run=TestHelperProcess",
		"--",
		"fail",
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}

	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Execute() error = %T, want *ExitError", err)
	}
	if exitErr.ExitCode != 7 {
		t.Fatalf("ExitError.ExitCode = %d, want 7", exitErr.ExitCode)
	}
	if result.ExitCode != 7 {
		t.Fatalf("Result.ExitCode = %d, want 7", result.ExitCode)
	}
	if result.Output != "failed\n" {
		t.Fatalf("Result.Output = %q, want %q", result.Output, "failed\n")
	}
}

func TestExecuteReturnsTimeoutError(t *testing.T) {
	withHelperProcess(t)

	result, err := Execute(context.Background(), &config.Tool{
		Command: os.Args[0],
		Timeout: "10ms",
	}, []string{
		"-test.run=TestHelperProcess",
		"--",
		"sleep",
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}

	var timeoutErr *TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("Execute() error = %T, want *TimeoutError", err)
	}
	if !result.TimedOut {
		t.Fatal("Result.TimedOut = false, want true")
	}
}

func TestExecuteAllowsConfiguredSuccessExitCode(t *testing.T) {
	withHelperProcess(t)

	result, err := Execute(context.Background(), &config.Tool{
		Command:          os.Args[0],
		SuccessExitCodes: []int{7},
	}, []string{
		"-test.run=TestHelperProcess",
		"--",
		"fail",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("Result.ExitCode = %d, want 7", result.ExitCode)
	}
}

func TestExecuteReturnsMissingRuntimeError(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	_, err := Execute(context.Background(), &config.Tool{
		Command: "echo",
		Docker:  &config.DockerConfig{Image: "alpine"},
	}, []string{"ok"})
	if err == nil {
		t.Fatal("Execute() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "container runtime not found") {
		t.Fatalf("Execute() error = %q, want container runtime error", err)
	}
}

func TestExecuteTruncatesOutput(t *testing.T) {
	withHelperProcess(t)

	result, err := Execute(context.Background(), &config.Tool{
		Command:        os.Args[0],
		MaxOutputBytes: 4,
	}, []string{
		"-test.run=TestHelperProcess",
		"--",
		"long-output",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !result.OutputTruncated {
		t.Fatal("Result.OutputTruncated = false, want true")
	}
	if !strings.HasPrefix(result.Output, "aaaa\n[WARN]") {
		t.Fatalf("Result.Output = %q, want truncated output with warning", result.Output)
	}
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("MCPWN_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for len(args) > 0 && args[0] != "--" {
		args = args[1:]
	}
	if len(args) < 2 {
		os.Exit(2)
	}

	switch args[1] {
	case "success":
		_, _ = os.Stdout.WriteString("ok\n")
		os.Exit(0)
	case "fail":
		_, _ = os.Stderr.WriteString("failed\n")
		os.Exit(7)
	case "sleep":
		time.Sleep(time.Second)
		os.Exit(0)
	case "long-output":
		_, _ = os.Stdout.WriteString("aaaaa")
		os.Exit(0)
	default:
		os.Exit(2)
	}
}

func withHelperProcess(t *testing.T) {
	t.Helper()

	t.Setenv("MCPWN_HELPER_PROCESS", "1")
}
