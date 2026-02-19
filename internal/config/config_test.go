package config

import (
	"log/slog"
	"os"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	yamlContent := `
tools:
  - name: "nmap_scan"
    description: "Nmap scan"
    command: "nmap"
    image: "parrotsec/nmap"
    args:
      - name: "target"
        description: "Target IP"
        required: true
        positional: true
`
	tmpfile, err := os.CreateTemp("", "mcpwn_test_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			slog.Error("Failed to remove temporary file")
		}
	}(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(yamlContent)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(cfg.Tools))
	}

	tool := cfg.Tools[0]
	if tool.Name != "nmap_scan" {
		t.Errorf("Expected tool name 'nmap_scan', got %q", tool.Name)
	}

	if tool.Description != "Nmap scan" {
		t.Errorf("Expected tool description 'Nmap scan', got %q", tool.Description)
	}

	if tool.Image != "parrotsec/nmap" {
		t.Errorf("Expected tool image 'parrotsec/nmap', got %q", tool.Image)
	}

	if len(tool.Args) != 1 {
		t.Errorf("Expected 1 arg, got %d", len(tool.Args))
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := Load("/non/existent/path.yaml")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}
