package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReadsValidConfig(t *testing.T) {
	path := writeConfig(t, `
tools:
  - name: echo
    description: Test tool
    command: echo
    fixed_args: ["--plain"]
    success_exit_codes: [0, 2]
    timeout: "30s"
    max_output_bytes: 4096
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

func TestLoadReadsDefaultConfig(t *testing.T) {
	cfg, err := Load(filepath.Join("..", "..", "mcpwn.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(cfg.Tools) == 0 {
		t.Fatal("len(cfg.Tools) = 0, want default tools")
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

func TestLoadRejectsUnknownField(t *testing.T) {
	path := writeConfig(t, `
tools:
  - name: echo
    description: Test tool
    command: echo
    unexpected: true
`)

	expectLoadErrorContains(t, path, "field unexpected not found")
}

func TestLoadRejectsEmptyTools(t *testing.T) {
	path := writeConfig(t, "tools: []")

	expectLoadErrorContains(t, path, "at least one tool is required")
}

func TestLoadRejectsDuplicateToolNames(t *testing.T) {
	path := writeConfig(t, `
tools:
  - name: echo
    description: Test tool
    command: echo
  - name: echo
    description: Duplicate tool
    command: echo
`)

	expectLoadErrorContains(t, path, "duplicate tool name")
}

func TestLoadRejectsInvalidTool(t *testing.T) {
	path := writeConfig(t, `
tools:
  - name: echo
    description: Test tool
    command: ""
`)

	expectLoadErrorContains(t, path, "empty command")
}

func TestLoadRejectsDockerConfigWithoutImage(t *testing.T) {
	path := writeConfig(t, `
tools:
  - name: echo
    description: Test tool
    command: echo
    docker: {}
`)

	expectLoadErrorContains(t, path, "docker config without image")
}

func TestLoadRejectsInvalidExecutionSettings(t *testing.T) {
	path := writeConfig(t, `
tools:
  - name: echo
    description: Test tool
    command: echo
    timeout: "-1s"
`)

	expectLoadErrorContains(t, path, "timeout must be greater than zero")
}

func TestLoadRejectsInvalidSuccessExitCode(t *testing.T) {
	path := writeConfig(t, `
tools:
  - name: echo
    description: Test tool
    command: echo
    success_exit_codes: [256]
`)

	expectLoadErrorContains(t, path, "invalid success exit code")
}

func TestLoadRejectsInvalidArg(t *testing.T) {
	path := writeConfig(t, `
tools:
  - name: echo
    description: Test tool
    command: echo
    args:
      - name: verbose
        description: Verbose output
        type: boolean
`)

	expectLoadErrorContains(t, path, "requires a flag")
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "mcpwn.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}

func expectLoadErrorContains(t *testing.T, path, want string) {
	t.Helper()

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want error")
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("Load() error = %q, want it to contain %q", err, want)
	}
}
