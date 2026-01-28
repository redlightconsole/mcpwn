package main

import (
	"log/slog"
	"mcpwn/internal/config"
	"mcpwn/internal/server"
	"os"
	"path/filepath"
	"runtime/debug"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
	Contact = "Dario Camonita <danterolle@parrotsec.org>"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	// Get version info from build metadata if ldflags didn't provide it
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				Commit = setting.Value
			}
			if setting.Key == "vcs.time" {
				Date = setting.Value
			}
		}
	}

	exePath, err := os.Executable()
	if err != nil {
		slog.Error("Failed to get executable path", "error", err)
		os.Exit(1)
	}
	configPath := filepath.Join(filepath.Dir(exePath), "mcpwn.yaml")

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("Configuration load error...", "path", configPath, "error", err)
		os.Exit(1)
	}

	slog.Info("mcpwn starting",
		"contact", Contact,
		"version", Version,
		"commit", Commit,
		"build_date", Date,
		"tools_count", len(cfg.Tools),
	)
	srv := server.New(cfg, Version)
	if err := srv.Serve(); err != nil {
		slog.Error("Server crash...", "error", err)
		os.Exit(1)
	}
}
