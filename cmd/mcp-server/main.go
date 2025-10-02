package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"mcpwn/internal/client"
	"net/http"
	"time"
)

func createToolProxyHandler(Client *client.Client, apiEndpoint string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var params map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		result, err := Client.Post(apiEndpoint, params)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result) // TODO: Handle error for Encoding
	}
}

func main() {
	serverURL := flag.String("server", "http://localhost:5000", "API server URL")
	timeoutSec := flag.Int("timeout", 300, "Request timeout in seconds")
	mcpPort := flag.Int("port", 8000, "Port for the MCP server")
	flag.Parse()

	Client := client.New(*serverURL, time.Duration(*timeoutSec)*time.Second)

	health, err := Client.CheckHealth()
	if err != nil {
		slog.Warn("Unable to connect to the API server", "server", *serverURL, "err", err)
	} else {
		slog.Info("Successfully connected to the API server", "status", health.Status)
		if !health.AllMainToolsAvailable {
			slog.Warn("Not all main tools are available on the server.")
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/tools/nmap", createToolProxyHandler(Client, "api/tools/nmap"))
	mux.HandleFunc("/tools/command", createToolProxyHandler(Client, "api/command"))
	// Add more tool handlers as needed

	slog.Info("Starting MCP server on port", "port", *mcpPort)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", *mcpPort), mux); err != nil {
		slog.Error("Failed to start MCP server", "error", err)
	}
}
