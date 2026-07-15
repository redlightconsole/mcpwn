package executor

import (
	"context"
	"os"
	"testing"

	"mcpwn/internal/config"
)

func TestExecuteRunsCommand(t *testing.T) {
	withHelperProcess(t)

	output, err := Execute(context.Background(), &config.Tool{Command: os.Args[0]}, []string{
		"-test.run=TestHelperProcess",
		"--",
		"success",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if output != "ok\n" {
		t.Fatalf("Execute() output = %q, want %q", output, "ok\n")
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
	default:
		os.Exit(2)
	}
}

func withHelperProcess(t *testing.T) {
	t.Helper()

	t.Setenv("MCPWN_HELPER_PROCESS", "1")
}
