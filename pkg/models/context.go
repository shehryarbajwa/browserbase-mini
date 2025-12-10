package models

import "time"

// Context represents a persistent browser state
type Context struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	DataPath  string    `json:"-"` // Path to stored data (internal only)
}

// CreateContextRequest is the payload for creating a context
type CreateContextRequest struct {
	ProjectID string `json:"projectId"`
}

// CreateContextResponse includes upload credentials
type CreateContextResponse struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	CreatedAt time.Time `json:"createdAt"`
	// In real Browserbase, these would be for encrypted upload
	// For now, we'll handle it internally
}
