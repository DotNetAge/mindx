package auth

import (
	"fmt"
	"time"

	"crypto/rand"

	"golang.org/x/crypto/bcrypt"
	"mindx/internal/entity"
	"mindx/pkg/i18n"
	"mindx/pkg/logging"
)

// Service implements the AuthenticationService interface
type Service struct {
	jwtManager     *JWTManager
	apiKeyManager  *APIKeyManager
	users          map[string]*entity.User // username -> user
	logger         logging.Logger
	passwordPolicy PasswordPolicy
}

// PasswordPolicy defines password requirements
type PasswordPolicy struct {
	MinLength       int
	RequireUppercase bool
	RequireLowercase bool
	RequireNumbers   bool
	RequireSpecial   bool
}

// Config holds authentication configuration
type Config struct {
	JWTSecret        string
	TokenExpiration  time.Duration
	RefreshExpiration time.Duration
	PasswordPolicy   PasswordPolicy
}

// NewService creates a new authentication service
func NewService(config Config, logger logging.Logger) (*Service, error) {
	jwtManager, err := NewJWTManager(config.JWTSecret, config.TokenExpiration, config.RefreshExpiration)
	if err != nil {
		return nil, err
	}

	service := &Service{
		jwtManager:     jwtManager,
		apiKeyManager:  NewAPIKeyManager(),
		users:          make(map[string]*entity.User),
		logger:         logger.Named("AuthService"),
		passwordPolicy: config.PasswordPolicy,
	}

	// Create default admin user if no users exist
	if err := service.initDefaultUser(); err != nil {
		return nil, err
	}

	return service, nil
}

// initDefaultUser creates the default admin user
func (s *Service) initDefaultUser() error {
	if len(s.users) > 0 {
		return nil // Users already exist
	}

	// Generate random password
	password := generateRandomPassword(16)

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	// Create admin user
	user := &entity.User{
		ID:           generateUserID(),
		Username:     entity.DefaultUser,
		PasswordHash: string(hashedPassword),
		APIKeys:      []string{},
		CreatedAt:    time.Now(),
		LastLogin:    time.Now(),
		Active:       true,
	}

	s.users[user.Username] = user

	// Log the initial password (user should change it)
	s.logger.Warn("=============================================")
	s.logger.Warn("INITIAL ADMIN CREDENTIALS")
	s.logger.Warn("Username: admin")
	s.logger.Warn("Password: "+password)
	s.logger.Warn("IMPORTANT: Change this password immediately!")
	s.logger.Warn("Use: curl -X POST http://localhost:1314/api/auth/change-password \\\\n  -H \"Content-Type: application/json\" \\\n  -d '{\"old_password\":\""+password+"\",\"new_password\":\"YOUR_NEW_PASSWORD\"}'")
	s.logger.Warn("=============================================")

	return nil
}

// Login authenticates a user
func (s *Service) Login(username, password string) (string, string, error) {
	user, exists := s.users[username]
	if !exists {
		return "", "", fmt.Errorf(i18n.T("auth.invalid_credentials"))
	}

	if !user.Active {
		return "", "", fmt.Errorf(i18n.T("auth.account_inactive"))
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", "", fmt.Errorf(i18n.T("auth.invalid_credentials"))
	}

	// Update last login
	user.LastLogin = time.Now()

	// Generate JWT token
	token, err := s.jwtManager.GenerateToken(user)
	if err != nil {
		return "", "", err
	}

	// Generate API key if user doesn't have one
	var apiKey string
	if len(user.APIKeys) == 0 {
		generatedKey, err := s.apiKeyManager.GenerateKey(user)
		if err != nil {
			s.logger.Warn("Failed to generate API key", logging.Err(err))
			// Continue without API key
		} else {
			user.APIKeys = append(user.APIKeys, generatedKey)
			apiKey = generatedKey
		}
	} else {
		apiKey = user.APIKeys[0]
	}

	s.logger.Info(i18n.T("auth.login_success"), logging.String(i18n.T("auth.username"), username))

	return token, apiKey, nil
}

// ValidateJWT validates a JWT token
func (s *Service) ValidateJWT(token string) (*entity.User, error) {
	claims, err := s.jwtManager.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	user, exists := s.users[claims.Username]
	if !exists {
		return nil, fmt.Errorf(i18n.T("auth.user_not_found"))
	}

	if !user.Active {
		return nil, fmt.Errorf(i18n.T("auth.account_inactive"))
	}

	return user, nil
}

// ValidateAPIKey validates an API key
func (s *Service) ValidateAPIKey(apiKey string) (*entity.User, error) {
	userID, err := s.apiKeyManager.ValidateKey(apiKey)
	if err != nil {
		return nil, err
	}

	// Find user by ID
	for _, user := range s.users {
		if user.ID == userID {
			if !user.Active {
				return nil, fmt.Errorf(i18n.T("auth.account_inactive"))
			}
			return user, nil
		}
	}

	return nil, fmt.Errorf(i18n.T("auth.user_not_found"))
}

// CreateUser creates a new user
func (s *Service) CreateUser(username, password string) (*entity.User, error) {
	if _, exists := s.users[username]; exists {
		return nil, fmt.Errorf(i18n.T("auth.user_exists"))
	}

	if err := s.validatePassword(password); err != nil {
		return nil, err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &entity.User{
		ID:           generateUserID(),
		Username:     username,
		PasswordHash: string(hashedPassword),
		APIKeys:      []string{},
		CreatedAt:    time.Now(),
		LastLogin:    time.Now(),
		Active:       true,
	}

	s.users[user.Username] = user

	s.logger.Info(i18n.T("auth.user_created"), logging.String(i18n.T("auth.username"), username))

	return user, nil
}

// ChangePassword changes a user's password
func (s *Service) ChangePassword(userID, oldPassword, newPassword string) error {
	user, err := s.getUserByID(userID)
	if err != nil {
		return err
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return fmt.Errorf(i18n.T("auth.invalid_old_password"))
	}

	// Validate new password
	if err := s.validatePassword(newPassword); err != nil {
		return err
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedPassword)

	s.logger.Info(i18n.T("auth.password_changed"), logging.String(i18n.T("auth.username"), user.Username))

	return nil
}

// GenerateAPIKey generates a new API key for a user
func (s *Service) GenerateAPIKey(userID string) (string, error) {
	user, err := s.getUserByID(userID)
	if err != nil {
		return "", err
	}

	apiKey, err := s.apiKeyManager.GenerateKey(user)
	if err != nil {
		return "", err
	}

	user.APIKeys = append(user.APIKeys, apiKey)

	s.logger.Info(i18n.T("auth.apikey_generated"), logging.String(i18n.T("auth.username"), user.Username))

	return apiKey, nil
}

// RevokeAPIKey revokes an API key
func (s *Service) RevokeAPIKey(userID, apiKey string) error {
	user, err := s.getUserByID(userID)
	if err != nil {
		return err
	}

	// Check if user owns this key
	ownsKey := false
	for i, key := range user.APIKeys {
		if key == apiKey {
			ownsKey = true
			// Remove from slice
			user.APIKeys = append(user.APIKeys[:i], user.APIKeys[i+1:]...)
			break
		}
	}

	if !ownsKey {
		return fmt.Errorf(i18n.T("auth.apikey_not_found"))
	}

	if err := s.apiKeyManager.RevokeKey(apiKey); err != nil {
		return err
	}

	s.logger.Info(i18n.T("auth.apikey_revoked"), logging.String(i18n.T("auth.username"), user.Username))

	return nil
}

// GetUser retrieves a user by ID
func (s *Service) GetUser(userID string) (*entity.User, error) {
	return s.getUserByID(userID)
}

// GetUserByUsername retrieves a user by username
func (s *Service) GetUserByUsername(username string) (*entity.User, error) {
	user, exists := s.users[username]
	if !exists {
		return nil, fmt.Errorf(i18n.T("auth.user_not_found"))
	}
	return user, nil
}

// Helper functions

func (s *Service) getUserByID(userID string) (*entity.User, error) {
	for _, user := range s.users {
		if user.ID == userID {
			return user, nil
		}
	}
	return nil, fmt.Errorf(i18n.T("auth.user_not_found"))
}

func (s *Service) validatePassword(password string) error {
	if s.passwordPolicy.MinLength > 0 && len(password) < s.passwordPolicy.MinLength {
		return fmt.Errorf("password must be at least %d characters", s.passwordPolicy.MinLength)
	}

	// Add more validation as needed based on policy
	return nil
}

func generateUserID() string {
	return fmt.Sprintf("user_%d", time.Now().UnixNano())
}

func generateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()"

	b := make([]byte, length)
	buf := make([]byte, 1)
	for i := range b {
		_, err := rand.Read(buf)
		if err != nil {
			// Fallback to time-based
			b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
			continue
		}
		b[i] = charset[buf[0]%byte(len(charset))]
	}
	return string(b)
}
