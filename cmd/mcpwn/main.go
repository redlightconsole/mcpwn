package main

import (
	"flag"
	"fmt"
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
	transport := flag.String("transport", "stdio", "Transport to use: stdio or http")
	port := flag.Int("port", 3000, "Port to listen on (http transport only)")
	host := flag.String("host", "", "Host/interface to bind (http transport only; empty = all interfaces)")
	flag.Parse()

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

	// Try local config first, then /etc as a fallback option.
	configPath := filepath.Join(filepath.Dir(exePath), "mcpwn.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "/etc/mcpwn/mcpwn.yaml"
	}

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

	var serveErr error
	switch *transport {
	case "stdio":
		serveErr = srv.Serve()
	case "http":
		addr := fmt.Sprintf("%s:%d", *host, *port)
		serveErr = srv.ServeHTTP(addr)
	default:
		slog.Error("Unknown transport", "transport", *transport)
		os.Exit(1)
	}
	if serveErr != nil {
		slog.Error("Server crash...", "error", serveErr)
		os.Exit(1)
	}
}
