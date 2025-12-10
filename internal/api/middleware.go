package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/shehryarbajwa/browserbase-mini/internal/ratelimit"
)

// RateLimitMiddleware creates a middleware that enforces rate limits
func RateLimitMiddleware(limiter *ratelimit.Limiter, requestsPerHour int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract projectId from request body or query params
			projectID := getProjectID(r)

			if projectID == "" {
				// No project ID, skip rate limiting
				next.ServeHTTP(w, r)
				return
			}

			// Check rate limit
			if !limiter.Allow(projectID) {
				// Rate limit exceeded
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(requestsPerHour))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.WriteHeader(http.StatusTooManyRequests)

				json.NewEncoder(w).Encode(map[string]string{
					"error": "Rate limit exceeded. Maximum 100 requests per hour per project.",
				})
				return
			}

			// Add rate limit headers
			tokens := limiter.Tokens(projectID)
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(requestsPerHour))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(int(tokens)))

			// Request allowed, continue
			next.ServeHTTP(w, r)
		})
	}
}

// getProjectID extracts the project ID from the request
func getProjectID(r *http.Request) string {
	// Check query parameter first (for GET requests)
	projectID := r.URL.Query().Get("projectId")
	if projectID != "" {
		return projectID
	}

	// Could also check custom header
	projectID = r.Header.Get("X-Project-ID")
	if projectID != "" {
		return projectID
	}

	return ""
}
