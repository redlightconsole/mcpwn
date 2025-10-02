package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"mcpwn/internal/command"
	"mcpwn/internal/models"
	"net/http"
	"os/exec"
	"time"
)

type APIHandler struct {
	CommandTimeout time.Duration
}

func NewAPIHandler(timeout time.Duration) *APIHandler {
	return &APIHandler{CommandTimeout: timeout}
}

// TODO: this should be removed to avoid RCE.
func (h *APIHandler) genericCommandHandler(w http.ResponseWriter, r *http.Request) {
	var req models.GenericCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}
	if req.Command == "" {
		writeJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Command parameter is required"})
		return
	}
	result := command.Execute(req.Command, h.CommandTimeout)
	writeJSONResponse(w, http.StatusOK, result)
}

func (h *APIHandler) nmapHandler(w http.ResponseWriter, r *http.Request) {
	var req models.NmapRequest
	// TODO: should be configurable?
	req.ScanType = "-sCV"
	req.AdditionalArgs = "-T4 -Pn"

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}
	if req.Target == "" {
		writeJSONResponse(w, http.StatusBadRequest, map[string]string{"error": "Target parameter is required"})
		return
	}

	cmdStr := fmt.Sprintf("nmap %s %s -p %s %s", req.ScanType, req.AdditionalArgs, req.Ports, req.Target)
	result := command.Execute(cmdStr, h.CommandTimeout)
	writeJSONResponse(w, http.StatusOK, result)
}

func (h *APIHandler) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	mainTools := []string{"nmap", "gobuster"}
	toolsStatus := make(map[string]bool)
	allAvailable := true

	for _, tool := range mainTools {
		_, err := exec.LookPath(tool)
		if err != nil {
			toolsStatus[tool] = false
			allAvailable = false
		} else {
			toolsStatus[tool] = true
		}
	}

	response := models.HealthStatus{
		Status:                "healthy",
		Message:               "API Server is running",
		ToolsStatus:           toolsStatus,
		AllMainToolsAvailable: allAvailable,
	}
	writeJSONResponse(w, http.StatusOK, response)
}

func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("Error writing JSON response", "error", err)
	}
}
