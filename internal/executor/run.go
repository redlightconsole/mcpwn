package executor

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// SafeExecute runs a command and returns its combined output (stdout + stderr).
// It uses a timeout to prevent tools from hanging indefinitely.
// If an image is provided, it runs the command inside a Docker container.
func SafeExecute(ctx context.Context, command string, args []string, image string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

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
	} else {
		cmd = exec.CommandContext(ctx, command, args...)
	}

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
