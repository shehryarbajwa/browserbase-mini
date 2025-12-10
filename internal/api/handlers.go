package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/shehryarbajwa/browserbase-mini/internal/session"
	"github.com/shehryarbajwa/browserbase-mini/pkg/models"
)

// Handler holds dependencies for HTTP handlers
type Handler struct {
	sessionMgr *session.Manager
}

// NewHandler creates a new HTTP handler
func NewHandler(sessionMgr *session.Manager) *Handler {
	return &Handler{
		sessionMgr: sessionMgr,
	}
}

// CreateSession handles POST /v1/sessions
func (h *Handler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req models.CreateSessionRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	session, err := h.sessionMgr.CreateSession(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(session)
}

// GetSession handles GET /v1/sessions/{id}
func (h *Handler) GetSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	session, err := h.sessionMgr.GetSession(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// ListSessions handles GET /v1/sessions
func (h *Handler) ListSessions(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("projectId")
	statusStr := r.URL.Query().Get("status")

	var status models.SessionStatus
	if statusStr != "" {
		status = models.SessionStatus(statusStr)
	}

	sessions := h.sessionMgr.ListSessions(projectID, status)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

// DeleteSession handles DELETE /v1/sessions/{id}
func (h *Handler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := h.sessionMgr.DeleteSession(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetDebugURL handles GET /v1/sessions/{id}/debug
func (h *Handler) GetDebugURL(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	session, err := h.sessionMgr.GetSession(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	debugURL := fmt.Sprintf("ws://%s/v1/sessions/%s/ws", r.Host, session.ID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"debuggerUrl": debugURL,
		"sessionId":   session.ID,
		"status":      string(session.Status),
	})
}

// GetSessionScreenshot handles GET /v1/sessions/{id}/screenshot
func (h *Handler) GetSessionScreenshot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	session, err := h.sessionMgr.GetSession(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	if session.Status != "RUNNING" {
		http.Error(w, "Session is not running", http.StatusBadRequest)
		return
	}

	// Get persistent Puppeteer connection
	conn := h.sessionMgr.GetPuppeteerConnection(sessionID)
	if conn == nil {
		http.Error(w, "No Puppeteer connection available", http.StatusInternalServerError)
		return
	}

	// Send screenshot command
	result, err := conn.SendCommand(map[string]string{
		"action": "screenshot",
	}, 10*time.Second)

	if err != nil {
		log.Printf("❌ Screenshot failed: %v", err)
		http.Error(w, fmt.Sprintf("Screenshot failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Get base64 screenshot from result
	screenshotBase64, ok := result["data"].(string)
	if !ok {
		http.Error(w, "Invalid screenshot data", http.StatusInternalServerError)
		return
	}

	// Decode base64 to bytes
	screenshotBytes, err := base64.StdEncoding.DecodeString(screenshotBase64)
	if err != nil {
		http.Error(w, "Failed to decode screenshot", http.StatusInternalServerError)
		return
	}

	// Return PNG image
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Write(screenshotBytes)
}

// NavigateSession handles POST /v1/sessions/{id}/navigate
// NavigateSession handles POST /v1/sessions/{id}/navigate
func (h *Handler) NavigateSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["id"]

	var req struct {
		URL string `json:"url"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	session, err := h.sessionMgr.GetSession(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	if session.Status != "RUNNING" {
		http.Error(w, "Session not running", http.StatusBadRequest)
		return
	}

	// Get persistent Puppeteer connection
	conn := h.sessionMgr.GetPuppeteerConnection(sessionID)
	if conn == nil {
		http.Error(w, "No Puppeteer connection available", http.StatusInternalServerError)
		return
	}

	log.Printf("🚀 Navigating session %s to %s", sessionID[:8], req.URL)

	// Send navigate command
	result, err := conn.SendCommand(map[string]string{
		"action": "navigate",
		"url":    req.URL,
	}, 35*time.Second)

	if err != nil {
		log.Printf("❌ Navigation failed: %v", err)
		http.Error(w, fmt.Sprintf("Navigation failed: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Navigated to: %s", req.URL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"url":    result["url"],
	})
}
