package api

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	contextmgr "github.com/shehryarbajwa/browserbase-mini/internal/context"
	"github.com/shehryarbajwa/browserbase-mini/pkg/models"
)

// ContextHandler holds dependencies for context HTTP handlers
type ContextHandler struct {
	contextMgr *contextmgr.Manager
}

// NewContextHandler creates a new context HTTP handler
func NewContextHandler(contextMgr *contextmgr.Manager) *ContextHandler {
	return &ContextHandler{
		contextMgr: contextMgr,
	}
}

// CreateContext handles POST /v1/contexts
func (h *ContextHandler) CreateContext(w http.ResponseWriter, r *http.Request) {
	var req models.CreateContextRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	context, err := h.contextMgr.CreateContext(req.ProjectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(context)
}

// GetContext handles GET /v1/contexts/{id}
func (h *ContextHandler) GetContext(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	context, err := h.contextMgr.GetContext(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(context)
}

// DeleteContext handles DELETE /v1/contexts/{id}
func (h *ContextHandler) DeleteContext(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if err := h.contextMgr.DeleteContext(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
