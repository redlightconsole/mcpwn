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

// Execute runs a command and returns its combined output (stdout + stderr).
// It uses a timeout to prevent tools from hanging indefinitely.
// If an image is provided, it runs the command inside a container (podman preferred, docker fallback).
//
// Note: exec.CommandContext does **not** invoke a shell, so arguments are passed
// directly to the process without shell interpretation.
func Execute(ctx context.Context, tool *config.Tool, args []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	image := ""
	if tool.Docker != nil {
		image = tool.Docker.Image
	}
	slog.DebugContext(ctx, "Preparing command execution", "command", tool.Command, "image", image, "args", args)

	var cmd *exec.Cmd
	if tool.Docker != nil && tool.Docker.Image != "" {
		runtime := detectRuntime()
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
		slog.DebugContext(ctx, "Running inside container", "runtime", runtime, "container_args", dockerArgs)
	} else {
		cmd = exec.CommandContext(ctx, tool.Command, args...)
	}

	// Combine stdout and stderr to give the LLM full visibility on errors
	output, err := cmd.CombinedOutput()

	result := string(output)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			slog.WarnContext(ctx, "Command execution timed out", "command", tool.Command)
			return result + "\n[ERROR] Command exceeded execution time limit.", nil
		}
		slog.ErrorContext(ctx, "Command execution failed", "command", tool.Command, "error", err)
		return fmt.Sprintf("Exit Code Error: %v\nOutput:\n%s", err, result), nil
	}

	slog.DebugContext(ctx, "Command execution completed successfully", "command", tool.Command)
	return result, nil
}

// detectRuntime checks for available container runtimes, preferring podman over docker.
func detectRuntime() string {
	for _, runtime := range []string{"podman", "docker"} {
		if _, err := exec.LookPath(runtime); err == nil {
			return runtime
		}
	}
	return "docker"
}
