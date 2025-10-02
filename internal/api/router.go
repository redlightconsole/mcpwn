package api

import (
	"net/http"
	"time"
)

func NewRouter(timeout time.Duration) *http.ServeMux {
	handler := NewAPIHandler(timeout)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/command", handler.genericCommandHandler)
	mux.HandleFunc("/api/tools/nmap", handler.nmapHandler)
	// More handlers can be added here
	mux.HandleFunc("/health", handler.healthCheckHandler)

	return mux
}
