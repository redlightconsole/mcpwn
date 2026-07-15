package config

import (
	"fmt"
	"log/slog"
	"os"

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
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse configuration: %w", err)
	}

	return &cfg, nil
}
