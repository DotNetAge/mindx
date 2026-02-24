package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
)

var (
	// ErrInvalidCiphertext is returned when the ciphertext format is invalid
	ErrInvalidCiphertext = errors.New("invalid ciphertext format")
	// ErrDecryptionFailed is returned when decryption fails
	ErrDecryptionFailed = errors.New("decryption failed")
)

// EncryptionManager handles encryption and decryption operations
type EncryptionManager struct {
	key []byte
}

// NewEncryptionManager creates a new encryption manager
// If keySource is empty, generates a new key and logs it
func NewEncryptionManager(keySource string) (*EncryptionManager, error) {
	var key []byte
	var err error

	if keySource == "" {
		// Generate new key
		key, err = generateRandomKey()
		if err != nil {
			return nil, fmt.Errorf("failed to generate encryption key: %w", err)
		}
		// Log the key for user to save
		fmt.Fprintf(os.Stderr, "\n=============================================\n")
		fmt.Fprintf(os.Stderr, "IMPORTANT: Save this encryption key:\n")
		fmt.Fprintf(os.Stderr, "%s\n", base64.StdEncoding.EncodeToString(key))
		fmt.Fprintf(os.Stderr, "=============================================\n\n")
	} else {
		// Use provided key
		key, err = base64.StdEncoding.DecodeString(keySource)
		if err != nil {
			return nil, fmt.Errorf("failed to decode encryption key: %w", err)
		}
	}

	// Validate key length (AES-256 requires 32 bytes)
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length: %d (must be 32 bytes for AES-256)", len(key))
	}

	return &EncryptionManager{key: key}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM
func (e *EncryptionManager) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode to base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
func (e *EncryptionManager) Decrypt(ciphertext string) (string, error) {
	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidCiphertext, err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("%w: ciphertext too short", ErrDecryptionFailed)
	}

	// Extract nonce and ciphertext
	nonce, cipherData := data[:nonceSize], data[nonceSize:]

	// Decrypt and authenticate
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return string(plaintext), nil
}

// GenerateKey generates a new random encryption key
func GenerateKey() (string, error) {
	key, err := generateRandomKey()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// DeriveKey derives a key from a password using SHA-256
func DeriveKey(password string, salt string) []byte {
	hasher := sha256.New()
	hasher.Write([]byte(password + salt))
	return hasher.Sum(nil)
}

// generateRandomKey generates a random 32-byte key
func generateRandomKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}
