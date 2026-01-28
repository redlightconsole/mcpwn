package executor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// SafeExecute runs a command and returns its combined output (stdout + stderr).
// It uses a timeout to prevent tools from hanging indefinitely.
// Also, it adds some basic security checks for command injection characters
// (even if exec.Command is generally safe).
func SafeExecute(ctx context.Context, command string, args []string) (string, error) {
	for _, arg := range args {
		if strings.ContainsAny(arg, ";&|`$") {
			return "", fmt.Errorf("unsafe characters detected in argument: %s", arg)
		}
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)

	// Combine stdout and stderr to give the LLM full visibility on errors
	output, err := cmd.CombinedOutput()

	result := string(output)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return result + "\n[ERROR] Command exceeded execution time limit.", nil
		}
		return fmt.Sprintf("Exit Code Error: %v\nOutput:\n%s", err, result), nil
	}

	return result, nil
}
