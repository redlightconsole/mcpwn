package executor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"time"
)

// SafeExecute runs a command and returns its combined output (stdout + stderr).
// It uses a timeout to prevent tools from hanging indefinitely.
// If an image is provided, it runs the command inside a Docker container.
//
// Note: exec.CommandContext does **not** invoke a shell, so arguments are passed
// directly to the process without shell interpretation.
func SafeExecute(ctx context.Context, command string, args []string, image string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	slog.DebugContext(ctx, "Preparing command execution", "command", command, "image", image, "args", args)

	var cmd *exec.Cmd
	if image != "" {
		dockerArgs := []string{
			"run", "--rm", "-i",
			"--cap-drop", "ALL",
			"--cap-add", "NET_RAW",
			"--cap-add", "NET_ADMIN",
			image, command,
		}
		dockerArgs = append(dockerArgs, args...)
		cmd = exec.CommandContext(ctx, "docker", dockerArgs...)
		slog.DebugContext(ctx, "Running inside docker", "docker_args", dockerArgs)
	} else {
		cmd = exec.CommandContext(ctx, command, args...)
	}

	// Combine stdout and stderr to give the LLM full visibility on errors
	output, err := cmd.CombinedOutput()

	result := string(output)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			slog.WarnContext(ctx, "Command execution timed out", "command", command)
			return result + "\n[ERROR] Command exceeded execution time limit.", nil
		}
		slog.ErrorContext(ctx, "Command execution failed", "command", command, "error", err)
		return fmt.Sprintf("Exit Code Error: %v\nOutput:\n%s", err, result), nil
	}

	slog.DebugContext(ctx, "Command execution completed successfully", "command", command)
	return result, nil
}
