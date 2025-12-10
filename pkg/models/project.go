package models

import "time"

// Project represents a customer project with resource limits
type Project struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Concurrency    int       `json:"concurrency"`
	DefaultTimeout int       `json:"defaultTimeout"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// ProjectUsage tracks resource consumption for a project
type ProjectUsage struct {
	ProjectID      string `json:"projectId"`
	BrowserMinutes int64  `json:"browserMinutes"`
	ActiveSessions int    `json:"activeSessions"`
}
