package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/shehryarbajwa/browserbase-mini/internal/proxy"
	"github.com/shehryarbajwa/browserbase-mini/internal/ratelimit"
)

// SetupRoutes configures all HTTP routes
func (h *Handler) SetupRoutes(contextHandler *ContextHandler, proxyServer *proxy.Server, rateLimiter *ratelimit.Limiter) *mux.Router {
	r := mux.NewRouter()

	// API v1 routes
	api := r.PathPrefix("/v1").Subrouter()

	// Apply rate limiting middleware to session endpoints
	rateLimitedAPI := api.PathPrefix("").Subrouter()
	rateLimitedAPI.Use(RateLimitMiddleware(rateLimiter, 100))

	// Session endpoints (rate limited)
	rateLimitedAPI.HandleFunc("/sessions", h.CreateSession).Methods("POST")
	rateLimitedAPI.HandleFunc("/sessions", h.ListSessions).Methods("GET")
	rateLimitedAPI.HandleFunc("/sessions/{id}", h.GetSession).Methods("GET")
	rateLimitedAPI.HandleFunc("/sessions/{id}", h.DeleteSession).Methods("DELETE")

	// Screenshot endpoint (not rate limited - frequent polling)
	api.HandleFunc("/sessions/{id}/screenshot", h.GetSessionScreenshot).Methods("GET")

	// Debug endpoints (not rate limited)
	api.HandleFunc("/sessions/{id}/debug", h.GetDebugURL).Methods("GET")
	api.HandleFunc("/sessions/{id}/ws", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		sessionID := vars["id"]
		proxyServer.HandleDebugConnection(w, r, sessionID)
	}).Methods("GET")
	api.HandleFunc("/sessions/{id}/navigate", h.NavigateSession).Methods("POST", "OPTIONS")

	// Context endpoints (not rate limited)
	api.HandleFunc("/contexts", contextHandler.CreateContext).Methods("POST")
	api.HandleFunc("/contexts/{id}", contextHandler.GetContext).Methods("GET")
	api.HandleFunc("/contexts/{id}", contextHandler.DeleteContext).Methods("DELETE")

	// CORS middleware
	r.Use(corsMiddleware)

	return r
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
