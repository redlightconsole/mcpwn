package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Tool struct {
	Name        string   `yaml:"name"`        // Tool name (used in MCP)
	Description string   `yaml:"description"` // Tool description
	Command     string   `yaml:"command"`     // Binary command to execute
	Image       string   `yaml:"image"`       // Docker image (optional)
	FixedArgs   []string `yaml:"fixed_args"`  // Arguments passed to the tool
	Args        []Arg    `yaml:"args"`        // Dynamic arguments mapped from MCP
}

type Arg struct {
	Name        string `yaml:"name"`        // Argument name in MCP
	Description string `yaml:"description"` // Description of the argument
	Flag        string `yaml:"flag"`        // CLI flag (e.g., -p, --url)
	Positional  bool   `yaml:"positional"`  // Whether it's a positional argument
	Required    bool   `yaml:"required"`    // Whether it's mandatory
	Type        string `yaml:"type"`        // data type: string or boolean
}

// Config list of configured tools
type Config struct {
	Tools []Tool `yaml:"tools"`
}

func Load(path string) (*Config, error) {
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
