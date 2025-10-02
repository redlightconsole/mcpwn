package command

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"mcpwn/internal/models"
	"os/exec"
	"syscall"
	"time"
)

func Execute(command string, timeout time.Duration) models.CommandResult {
	slog.Info("Executing command", "command", command)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", command)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err := cmd.Start()
	if err != nil {
		slog.Error("Error starting command", "error", err)
		return models.CommandResult{
			Stderr:     err.Error(),
			ReturnCode: -1,
			Success:    false,
		}
	}

	err = cmd.Wait()

	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()
	result := models.CommandResult{
		Stdout:   stdout,
		Stderr:   stderr,
		TimedOut: false,
	}

	if ctx.Err() == context.DeadlineExceeded {
		slog.Warn("Command timed out", "timeout", timeout)
		result.TimedOut = true
		result.ReturnCode = -1
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	} else if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			result.ReturnCode = exitError.Sys().(syscall.WaitStatus).ExitStatus()
		}
	} else {
		result.ReturnCode = 0
	}

	result.Success = result.ReturnCode == 0 || (result.TimedOut && (len(stdout) > 0 || len(stderr) > 0))
	result.PartialResults = result.TimedOut && (len(stdout) > 0 || len(stderr) > 0)

	return result
}
