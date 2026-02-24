package entity

import "time"

// User represents a user in the system
// For personal use scenario, there's typically just one admin user
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // Never expose in JSON
	APIKeys      []string  `json:"api_keys"`
	CreatedAt    time.Time `json:"created_at"`
	LastLogin    time.Time `json:"last_login"`
	Active       bool      `json:"active"`
}

// Default username for personal use
const DefaultUser = "admin"

// APIKeyPrefix is the prefix for generated API keys
const APIKeyPrefix = "sk-mindx-"
