package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadReadsValidConfig(t *testing.T) {
	path := writeConfig(t, `
tools:
  - name: echo
    description: Test tool
    command: echo
    fixed_args: ["--plain"]
    args:
      - name: message
        description: Message to print
        required: true
        positional: true
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Tools) != 1 {
		t.Fatalf("len(cfg.Tools) = %d, want 1", len(cfg.Tools))
	}
	if cfg.Tools[0].Name != "echo" {
		t.Fatalf("cfg.Tools[0].Name = %q, want %q", cfg.Tools[0].Name, "echo")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	path := writeConfig(t, "tools:\n  - name: [")

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "mcpwn.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}
