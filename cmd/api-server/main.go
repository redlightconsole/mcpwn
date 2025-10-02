package main

import (
	"flag"
	"fmt"
	"log/slog"
	"mcpwn/internal/api"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port           int
	CommandTimeout time.Duration
}

func main() {
	conf := &Config{}
	flag.IntVar(&conf.Port, "port", 5000, "Port for the API server")
	timeoutSec := flag.Int("timeout", 180, "Default command timeout in seconds")
	flag.Parse()
	conf.CommandTimeout = time.Duration(*timeoutSec) * time.Second

	if portStr, ok := os.LookupEnv("API_PORT"); ok {
		if p, err := strconv.Atoi(portStr); err == nil {
			conf.Port = p
		}
	}

	slog.Info("Starting the API Server on port", "port", conf.Port)
	slog.Info("Default command timeout is", "timeout", conf.CommandTimeout)

	router := api.NewRouter(conf.CommandTimeout)

	addr := fmt.Sprintf("0.0.0.0:%d", conf.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	if err := server.ListenAndServe(); err != nil {
		slog.Error("Failed to start server", "error", err)
	}
}
