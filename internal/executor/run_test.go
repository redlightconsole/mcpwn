package executor

import (
	"context"
	"strings"
	"testing"
)

func TestSafeExecute_UnsafeCharacters(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		wantErr bool
	}{
		{
			name:    "Safe arguments",
			command: "echo",
			args:    []string{"hello", "world"},
			wantErr: false,
		},
		{
			name:    "Unsafe semicolon",
			command: "echo",
			args:    []string{"hello;", "world"},
			wantErr: true,
		},
		{
			name:    "Unsafe pipe",
			command: "echo",
			args:    []string{"hello|world"},
			wantErr: true,
		},
		{
			name:    "Unsafe dollar",
			command: "echo",
			args:    []string{"$HOME"},
			wantErr: true,
		},
		{
			name:    "Unsafe backtick",
			command: "echo",
			args:    []string{"`whoami`"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SafeExecute(context.Background(), tt.command, tt.args, "")
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeExecute() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil {
				if !strings.Contains(err.Error(), "unsafe characters detected") {
					t.Errorf("SafeExecute() error = %v, want error to contain 'unsafe characters detected'", err)
				}
			}
		})
	}
}

func TestSafeExecute_SimpleCommand(t *testing.T) {
	ctx := context.Background()
	output, err := SafeExecute(ctx, "echo", []string{"hello"}, "")
	if err != nil {
		t.Fatalf("SafeExecute failed: %v", err)
	}

	expected := "hello"
	if !strings.Contains(output, expected) {
		t.Errorf("Expected output to contain %q, got %q", expected, output)
	}
}
