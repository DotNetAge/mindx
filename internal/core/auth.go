package core

import "mindx/internal/entity"

// AuthenticationService defines the interface for authentication operations
type AuthenticationService interface {
	// Login authenticates a user with username and password
	// Returns a JWT token and API key on success
	Login(username, password string) (token string, apiKey string, err error)

	// ValidateJWT validates a JWT token and returns the user if valid
	ValidateJWT(token string) (*entity.User, error)

	// ValidateAPIKey validates an API key and returns the user if valid
	ValidateAPIKey(apiKey string) (*entity.User, error)

	// CreateUser creates a new user (for admin operations)
	CreateUser(username, password string) (*entity.User, error)

	// ChangePassword changes a user's password
	ChangePassword(userID, oldPassword, newPassword string) error

	// GenerateAPIKey generates a new API key for a user
	GenerateAPIKey(userID string) (string, error)

	// RevokeAPIKey revokes an API key
	RevokeAPIKey(userID, apiKey string) error

	// GetUser retrieves a user by ID
	GetUser(userID string) (*entity.User, error)

	// GetUserByUsername retrieves a user by username
	GetUserByUsername(username string) (*entity.User, error)
}
