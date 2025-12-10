package models

import "time"

// SessionStatus represents the current state of a browser session
type SessionStatus string

const (
	StatusRunning   SessionStatus = "RUNNING"
	StatusCompleted SessionStatus = "COMPLETED"
	StatusError     SessionStatus = "ERROR"
	StatusTimedOut  SessionStatus = "TIMED_OUT"
)

// Session represents an active browser instance
type Session struct {
	ID          string        `json:"id"`
	ProjectID   string        `json:"projectId"`
	Status      SessionStatus `json:"status"`
	Region      string        `json:"region"`
	StartedAt   time.Time     `json:"startedAt"`
	ExpiresAt   time.Time     `json:"expiresAt"`
	Timeout     int           `json:"timeout"`
	ConnectURL  string        `json:"connectUrl"`
	ContainerID string        `json:"-"`
	ContextID   string        `json:"contextId,omitempty"`
	UserDataDir string        `json:"-"` // NEW: Track user data directory
}

// CreateSessionRequest is the payload for creating a new session
type CreateSessionRequest struct {
	ProjectID string `json:"projectId"`
	Region    string `json:"region,omitempty"`
	Timeout   int    `json:"timeout,omitempty"`
	ContextID string `json:"contextId,omitempty"`
}
