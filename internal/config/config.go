package config

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type DockerConfig struct {
	Image        string   `yaml:"image"`
	Capabilities []string `yaml:"capabilities"`
	Memory       string   `yaml:"memory"`
	CPUs         string   `yaml:"cpus"`
	Network      string   `yaml:"network"`
	TmpDirs      []string `yaml:"tmp_dirs"`
	Volumes      []string `yaml:"volumes"`
}

type Tool struct {
	Name             string        `yaml:"name"`               // Tool name (used in MCP)
	Description      string        `yaml:"description"`        // Tool description
	Command          string        `yaml:"command"`            // Binary command to execute
	Docker           *DockerConfig `yaml:"docker"`             // Container configuration (optional, uses podman or docker)
	FixedArgs        []string      `yaml:"fixed_args"`         // Arguments passed to the tool
	Args             []Arg         `yaml:"args"`               // Dynamic arguments mapped from MCP
	SuccessExitCodes []int         `yaml:"success_exit_codes"` // Exit codes treated as successful
	Timeout          string        `yaml:"timeout"`            // Execution timeout as a Go duration
	MaxOutputBytes   int64         `yaml:"max_output_bytes"`   // Maximum combined stdout/stderr bytes kept in memory
}

type Arg struct {
	Name        string `yaml:"name"`        // Argument name in MCP
	Description string `yaml:"description"` // Description of the argument
	Flag        string `yaml:"flag"`        // CLI flag (e.g., -p, --url)
	Positional  bool   `yaml:"positional"`  // Whether it's a positional argument
	Required    bool   `yaml:"required"`    // Whether it's mandatory
	Type        string `yaml:"type"`        // data type: string or boolean
}

type Config struct {
	Tools []Tool `yaml:"tools"`
}

func Load(path string) (*Config, error) {
	slog.Info("Loading configuration", "path", path)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

func (cfg *Config) Validate() error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if len(cfg.Tools) == 0 {
		return fmt.Errorf("at least one tool is required")
	}

	toolNames := make(map[string]struct{}, len(cfg.Tools))
	for i, tool := range cfg.Tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			return fmt.Errorf("tool at index %d has empty name", i)
		}
		if _, exists := toolNames[name]; exists {
			return fmt.Errorf("duplicate tool name: %s", name)
		}
		toolNames[name] = struct{}{}

		if strings.TrimSpace(tool.Command) == "" {
			return fmt.Errorf("tool %q has empty command", name)
		}
		if tool.Docker != nil && strings.TrimSpace(tool.Docker.Image) == "" {
			return fmt.Errorf("tool %q has docker config without image", name)
		}
		if tool.Timeout != "" {
			timeout, err := time.ParseDuration(tool.Timeout)
			if err != nil {
				return fmt.Errorf("tool %q has invalid timeout: %w", name, err)
			}
			if timeout <= 0 {
				return fmt.Errorf("tool %q timeout must be greater than zero", name)
			}
		}
		if tool.MaxOutputBytes < 0 {
			return fmt.Errorf("tool %q max_output_bytes must be zero or greater", name)
		}
		for _, code := range tool.SuccessExitCodes {
			if code < 0 || code > 255 {
				return fmt.Errorf("tool %q has invalid success exit code: %d", name, code)
			}
		}
		if err := validateArgs(name, tool.Args); err != nil {
			return err
		}
	}

	return nil
}

func validateArgs(toolName string, args []Arg) error {
	argNames := make(map[string]struct{}, len(args))
	for i, arg := range args {
		name := strings.TrimSpace(arg.Name)
		if name == "" {
			return fmt.Errorf("tool %q arg at index %d has empty name", toolName, i)
		}
		if _, exists := argNames[name]; exists {
			return fmt.Errorf("tool %q has duplicate arg name: %s", toolName, name)
		}
		argNames[name] = struct{}{}

		switch arg.Type {
		case "", "string", "boolean":
		default:
			return fmt.Errorf("tool %q arg %q has invalid type: %s", toolName, name, arg.Type)
		}
		if arg.Type == "boolean" && strings.TrimSpace(arg.Flag) == "" {
			return fmt.Errorf("tool %q boolean arg %q requires a flag", toolName, name)
		}
		if arg.Type == "boolean" && arg.Positional {
			return fmt.Errorf("tool %q boolean arg %q cannot be positional", toolName, name)
		}
		if arg.Positional && strings.TrimSpace(arg.Flag) != "" {
			return fmt.Errorf("tool %q positional arg %q cannot define a flag", toolName, name)
		}
	}

	return nil
}
