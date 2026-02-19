package executor

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestMultipleArgs(t *testing.T) {
	ctx := context.Background()
	output, err := SafeExecute(ctx, "echo", []string{"hello", "world", "foo"}, "")
	if err != nil {
		t.Fatalf("SafeExecute failed: %v", err)
	}

	if !strings.Contains(output, "hello world foo") {
		t.Errorf("Expected output to contain 'hello world foo', got %q", output)
	}
}

func TestEmptyArgs(t *testing.T) {
	ctx := context.Background()
	output, err := SafeExecute(ctx, "echo", []string{}, "")
	if err != nil {
		t.Fatalf("SafeExecute failed: %v", err)
	}

	if len(strings.TrimSpace(output)) != 0 {
		t.Errorf("Expected empty output from echo with no args, got %q", output)
	}
}

func TestTimeout(t *testing.T) {
	// Set a timeout shorter than the command execution time
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Sleep for 50ms, ensuring the 10ms timeout triggers
	output, err := SafeExecute(ctx, "sleep", []string{"0.05"}, "")
	if err != nil {
		t.Fatalf("SafeExecute should not return a Go-level error, got: %v", err)
	}

	expectedError := "Command exceeded execution time limit"
	if !strings.Contains(output, expectedError) {
		t.Errorf("Expected output to contain %q, got %q", expectedError, output)
	}
}
