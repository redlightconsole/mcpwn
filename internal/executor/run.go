package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"mcpwn/internal/config"
)

const (
	defaultTimeout        = 10 * time.Minute
	defaultMaxOutputBytes = 1024 * 1024
)

type Result struct {
	Output          string
	ExitCode        int
	TimedOut        bool
	OutputTruncated bool
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
	timeout, err := timeoutFor(tool)
	if err != nil {
		return Result{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
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

	output := newLimitedBuffer(maxOutputBytesFor(tool))
	cmd.Stdout = output
	cmd.Stderr = output
	err = cmd.Run()

	result := Result{
		Output:          output.String(),
		OutputTruncated: output.Truncated(),
	}
	if result.OutputTruncated {
		result.Output += fmt.Sprintf("\n[WARN] Command output exceeded %d bytes and was truncated.", output.Limit())
	}

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			slog.WarnContext(ctx, "Command execution timed out", "command", tool.Command)
			result.TimedOut = true
			return result, &TimeoutError{Command: tool.Command, Timeout: timeout}
		}

		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			if isSuccessExitCode(tool, result.ExitCode) {
				return result, nil
			}
			slog.ErrorContext(ctx, "Command execution failed", "command", tool.Command, "exit_code", result.ExitCode)
			return result, &ExitError{Command: tool.Command, ExitCode: result.ExitCode}
		}

		slog.ErrorContext(ctx, "Command execution failed", "command", tool.Command, "error", err)
		return result, fmt.Errorf("failed to execute %s: %w", tool.Command, err)
	}

	if !isSuccessExitCode(tool, 0) {
		return result, &ExitError{Command: tool.Command, ExitCode: 0}
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

func timeoutFor(tool *config.Tool) (time.Duration, error) {
	if tool.Timeout == "" {
		return defaultTimeout, nil
	}

	timeout, err := time.ParseDuration(tool.Timeout)
	if err != nil {
		return 0, fmt.Errorf("invalid timeout for %s: %w", tool.Name, err)
	}
	if timeout <= 0 {
		return 0, fmt.Errorf("invalid timeout for %s: must be greater than zero", tool.Name)
	}
	return timeout, nil
}

func maxOutputBytesFor(tool *config.Tool) int64 {
	if tool.MaxOutputBytes > 0 {
		return tool.MaxOutputBytes
	}
	return defaultMaxOutputBytes
}

func isSuccessExitCode(tool *config.Tool, code int) bool {
	if len(tool.SuccessExitCodes) == 0 {
		return code == 0
	}

	for _, successCode := range tool.SuccessExitCodes {
		if code == successCode {
			return true
		}
	}
	return false
}

type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int64
	truncated bool
}

func newLimitedBuffer(limit int64) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	originalLen := len(p)
	if b.limit <= 0 {
		_, err := b.buf.Write(p)
		return originalLen, err
	}

	remaining := b.limit - int64(b.buf.Len())
	if remaining <= 0 {
		b.truncated = true
		return originalLen, nil
	}

	if int64(len(p)) > remaining {
		p = p[:remaining]
		b.truncated = true
	}

	_, err := b.buf.Write(p)
	return originalLen, err
}

func (b *limitedBuffer) String() string {
	return b.buf.String()
}

func (b *limitedBuffer) Truncated() bool {
	return b.truncated
}

func (b *limitedBuffer) Limit() int64 {
	return b.limit
}
