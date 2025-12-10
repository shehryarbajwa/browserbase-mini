package ratelimit

import (
	"sync"

	"golang.org/x/time/rate"
)

// Limiter manages rate limits for multiple projects
type Limiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewLimiter creates a new rate limiter
// requestsPerHour: total requests allowed per hour per project (e.g., 100)
// burst: max requests in a burst (e.g., 10)
func NewLimiter(requestsPerHour int, burst int) *Limiter {
	// Convert requests per hour to requests per second
	r := rate.Limit(float64(requestsPerHour) / 3600.0)

	return &Limiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     r,
		burst:    burst,
	}
}

// GetLimiter returns the rate limiter for a specific project
func (l *Limiter) GetLimiter(projectID string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	limiter, exists := l.limiters[projectID]
	if !exists {
		limiter = rate.NewLimiter(l.rate, l.burst)
		l.limiters[projectID] = limiter
	}

	return limiter
}

// Allow checks if a request is allowed for the given project
func (l *Limiter) Allow(projectID string) bool {
	limiter := l.GetLimiter(projectID)
	return limiter.Allow()
}

// Tokens returns the current number of available tokens for a project
func (l *Limiter) Tokens(projectID string) float64 {
	limiter := l.GetLimiter(projectID)
	return limiter.Tokens()
}
