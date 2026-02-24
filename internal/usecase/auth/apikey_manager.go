package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"mindx/internal/entity"
)

// APIKeyManager handles API key generation and validation
type APIKeyManager struct {
	keys map[string]string // apiKey -> userID
}

// NewAPIKeyManager creates a new API key manager
func NewAPIKeyManager() *APIKeyManager {
	return &APIKeyManager{
		keys: make(map[string]string),
	}
}

// GenerateKey generates a new API key for a user
func (m *APIKeyManager) GenerateKey(user *entity.User) (string, error) {
	// Generate random bytes (32 bytes for 256-bit security)
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode to base64 and remove padding
	apiKey := base64.StdEncoding.EncodeToString(randomBytes)
	apiKey = strings.TrimRight(apiKey, "=")

	// Add prefix
	fullKey := fmt.Sprintf("%s%s", entity.APIKeyPrefix, apiKey)

	// Store mapping
	m.keys[fullKey] = user.ID

	return fullKey, nil
}

// ValidateKey validates an API key and returns the user ID
func (m *APIKeyManager) ValidateKey(apiKey string) (string, error) {
	// Check format
	if !isValidAPIKeyFormat(apiKey) {
		return "", fmt.Errorf("invalid API key format")
	}

	// Check if key exists
	userID, exists := m.keys[apiKey]
	if !exists {
		return "", fmt.Errorf("API key not found")
	}

	return userID, nil
}

// RevokeKey revokes an API key
func (m *APIKeyManager) RevokeKey(apiKey string) error {
	if _, exists := m.keys[apiKey]; !exists {
		return fmt.Errorf("API key not found")
	}

	delete(m.keys, apiKey)
	return nil
}

// LoadKeys loads existing API keys for a user
func (m *APIKeyManager) LoadKeys(userID string, apiKeys []string) {
	for _, key := range apiKeys {
		m.keys[key] = userID
	}
}

// isValidAPIKeyFormat checks if the API key has a valid format
func isValidAPIKeyFormat(apiKey string) bool {
	// Must start with prefix
	if !strings.HasPrefix(apiKey, entity.APIKeyPrefix) {
		return false
	}

	// Extract the actual key part
	keyPart := strings.TrimPrefix(apiKey, entity.APIKeyPrefix)

	// Must be base64-like (alphanumeric + +/ with possible trailing =)
	// After removing =, should be at least 32 characters
	matched, _ := regexp.MatchString(`^[A-Za-z0-9+/]{32,}$`, keyPart)
	return matched
}
