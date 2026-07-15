package executor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"mcpwn/internal/config"
)

const defaultTimeout = 10 * time.Minute

type Result struct {
	Output   string
	ExitCode int
	TimedOut bool
}

type ExitError struct {
	Command  string
	ExitCode int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("%s exited with code %d", e.Command, e.ExitCode)
}

type TimeoutError struct {
	Command string
	Timeout time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("%s exceeded execution time limit %s", e.Command, e.Timeout)
}

// Execute runs a command and returns its combined output (stdout + stderr).
// It uses a timeout to prevent tools from hanging indefinitely.
// If an image is provided, it runs the command inside a container (podman preferred, docker fallback).
//
// Note: exec.CommandContext does **not** invoke a shell, so arguments are passed
// directly to the process without shell interpretation.
func Execute(ctx context.Context, tool *config.Tool, args []string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	image := ""
	if tool.Docker != nil {
		image = tool.Docker.Image
	}
	slog.DebugContext(ctx, "Preparing command execution", "command", tool.Command, "image", image, "args_count", len(args))

	var cmd *exec.Cmd
	if tool.Docker != nil && tool.Docker.Image != "" {
		runtime, err := detectRuntime()
		if err != nil {
			return Result{}, err
		}
		dockerArgs := []string{
			"run", "--rm", "-i",
			"--security-opt", "no-new-privileges",
			"--read-only",
			"--cap-drop", "ALL",
		}

		if tool.Docker.Memory != "0" {
			mem := tool.Docker.Memory
			if mem == "" {
				mem = "512m"
			}
			dockerArgs = append(dockerArgs, "--memory", mem)
		}

		if tool.Docker.CPUs != "0" {
			cpus := tool.Docker.CPUs
			if cpus == "" {
				cpus = "1"
			}
			dockerArgs = append(dockerArgs, "--cpus", cpus)
		}

		if tool.Docker.Network != "" {
			dockerArgs = append(dockerArgs, "--network", tool.Docker.Network)
		}

		tmpDirs := tool.Docker.TmpDirs
		if len(tmpDirs) == 0 {
			tmpDirs = []string{"/tmp"}
		}
		for _, dir := range tmpDirs {
			// Using strings.Split to handle rw/ro options nicely if needed, but keeping it simple for now:
			dockerArgs = append(dockerArgs, "--tmpfs", dir)
		}

		for _, vol := range tool.Docker.Volumes {
			dockerArgs = append(dockerArgs, "-v", vol)
		}

		for _, cap := range tool.Docker.Capabilities {
			dockerArgs = append(dockerArgs, "--cap-add", cap)
		}
		dockerArgs = append(dockerArgs, tool.Docker.Image, tool.Command)
		dockerArgs = append(dockerArgs, args...)
		cmd = exec.CommandContext(ctx, runtime, dockerArgs...)
		slog.DebugContext(ctx, "Running inside container", "runtime", runtime, "image", tool.Docker.Image)
	} else {
		cmd = exec.CommandContext(ctx, tool.Command, args...)
	}

	// Combine stdout and stderr to give the LLM full visibility on errors
	output, err := cmd.CombinedOutput()

	result := Result{Output: string(output)}
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			slog.WarnContext(ctx, "Command execution timed out", "command", tool.Command)
			result.TimedOut = true
			return result, &TimeoutError{Command: tool.Command, Timeout: defaultTimeout}
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			slog.ErrorContext(ctx, "Command execution failed", "command", tool.Command, "exit_code", result.ExitCode)
			return result, &ExitError{Command: tool.Command, ExitCode: result.ExitCode}
		}

		slog.ErrorContext(ctx, "Command execution failed", "command", tool.Command, "error", err)
		return result, fmt.Errorf("failed to execute %s: %w", tool.Command, err)
	}

	slog.DebugContext(ctx, "Command execution completed successfully", "command", tool.Command)
	return result, nil
}

// detectRuntime checks for available container runtimes, preferring podman over docker.
func detectRuntime() (string, error) {
	for _, runtime := range []string{"podman", "docker"} {
		if _, err := exec.LookPath(runtime); err == nil {
			return runtime, nil
		}
	}
	return "", errors.New("container runtime not found: install podman or docker")
}
