package server

import (
	"reflect"
	"strings"
	"testing"

	"mcpwn/internal/config"
)

func TestBuildArgs(t *testing.T) {
	tool := &config.Tool{
		FixedArgs: []string{"scan"},
		Args: []config.Arg{
			{Name: "target", Required: true, Positional: true},
			{Name: "ports", Flag: "-p"},
			{Name: "verbose", Flag: "-v", Type: "boolean"},
			{Name: "dry_run", Flag: "--dry-run", Type: "boolean"},
			{Name: "extra_args", Flag: ""},
		},
	}

	args, err := buildArgs(tool, map[string]interface{}{
		"target":     "127.0.0.1",
		"ports":      "80,443",
		"verbose":    true,
		"dry_run":    false,
		"extra_args": "--reason --top-ports 10",
	})
	if err != nil {
		t.Fatalf("buildArgs() error = %v", err)
	}

	want := []string{"scan", "-p", "80,443", "-v", "--reason", "--top-ports", "10", "127.0.0.1"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("buildArgs() = %#v, want %#v", args, want)
	}
}

func TestBuildArgsMissingRequired(t *testing.T) {
	tool := &config.Tool{
		Args: []config.Arg{{Name: "target", Required: true}},
	}

	_, err := buildArgs(tool, map[string]interface{}{})
	if err == nil {
		t.Fatal("buildArgs() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "target") {
		t.Fatalf("buildArgs() error = %q, want it to mention target", err)
	}
}
