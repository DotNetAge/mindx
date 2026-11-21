package tokenmanager

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToken_IsExpired(t *testing.T) {
	tests := []struct {
		name     string
		token    *Token
		expected bool
	}{
		{
			name: "not expired",
			token: &Token{
				Value:     "test_token",
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
			expected: false,
		},
		{
			name: "expired",
			token: &Token{
				Value:     "test_token",
				ExpiresAt: time.Now().Add(-1 * time.Hour),
			},
			expected: true,
		},
		{
			name: "just expired",
			token: &Token{
				Value:     "test_token",
				ExpiresAt: time.Now().Add(-1 * time.Second),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.token.IsExpired())
		})
	}
}

func TestNewManager(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name:   "default config",
			config: Config{},
		},
		{
			name: "with initial token",
			config: Config{
				InitialToken:  "initial_token",
				InitialExpiry: time.Now().Add(1 * time.Hour),
			},
		},
		{
			name: "with refresh interval",
			config: Config{
				RefreshInterval: 30 * time.Minute,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewManager(tt.config)
			assert.NotNil(t, mgr)
			assert.NotNil(t, mgr.stopCh)
		})
	}
}

func TestManager_GetToken(t *testing.T) {
	tests := []struct {
		name      string
		token     *Token
		wantErr   error
		wantToken string
	}{
		{
			name: "valid token",
			token: &Token{
				Value:     "valid_token",
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
			wantErr:   nil,
			wantToken: "valid_token",
		},
		{
			name:      "no token",
			token:     nil,
			wantErr:   ErrTokenExpired,
			wantToken: "",
		},
		{
			name: "expired token",
			token: &Token{
				Value:     "expired_token",
				ExpiresAt: time.Now().Add(-1 * time.Hour),
			},
			wantErr:   ErrTokenExpired,
			wantToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewManager(Config{})
			mgr.token = tt.token

			token, err := mgr.GetToken()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantToken, token)
			}
		})
	}
}

func TestManager_Refresh(t *testing.T) {
	tests := []struct {
		name       string
		refreshFn  RefreshFunc
		wantErr    error
		checkToken bool
	}{
		{
			name: "successful refresh",
			refreshFn: func(ctx context.Context) (string, time.Duration, error) {
				return "new_token", 2 * time.Hour, nil
			},
			wantErr:    nil,
			checkToken: true,
		},
		{
			name:       "no refresh function",
			refreshFn:  nil,
			wantErr:    ErrNoRefreshFunc,
			checkToken: false,
		},
		{
			name: "refresh error",
			refreshFn: func(ctx context.Context) (string, time.Duration, error) {
				return "", 0, errors.New("refresh failed")
			},
			wantErr:    errors.New("refresh failed"),
			checkToken: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewManager(Config{
				RefreshFunc: tt.refreshFn,
			})

			err := mgr.Refresh(context.Background())
			if tt.wantErr != nil {
				if errors.Is(tt.wantErr, ErrNoRefreshFunc) {
					assert.ErrorIs(t, err, tt.wantErr)
				} else {
					assert.Error(t, err)
				}
			} else {
				assert.NoError(t, err)
				if tt.checkToken {
					token, err := mgr.GetToken()
					assert.NoError(t, err)
					assert.Equal(t, "new_token", token)
				}
			}
		})
	}
}

func TestManager_StartStop(t *testing.T) {
	refreshCount := 0
	refreshFn := func(ctx context.Context) (string, time.Duration, error) {
		refreshCount++
		return "token_" + string(rune('0'+refreshCount)), 2 * time.Hour, nil
	}

	mgr := NewManager(Config{
		RefreshFunc:     refreshFn,
		RefreshInterval: 100 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Start should trigger initial refresh
	err := mgr.Start(ctx)
	require.NoError(t, err)

	// Should have token after start
	token, err := mgr.GetToken()
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// Stop
	mgr.Stop()

	// Stop should be safe to call multiple times
	mgr.Stop()
}

func TestManager_Start_WithInitialToken(t *testing.T) {
	refreshFn := func(ctx context.Context) (string, time.Duration, error) {
		return "refreshed_token", 2 * time.Hour, nil
	}

	mgr := NewManager(Config{
		RefreshFunc:     refreshFn,
		RefreshInterval: 100 * time.Millisecond,
		InitialToken:    "initial_token",
		InitialExpiry:   time.Now().Add(1 * time.Hour),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start should not need to refresh if token is valid
	err := mgr.Start(ctx)
	require.NoError(t, err)

	token, err := mgr.GetToken()
	require.NoError(t, err)
	assert.Equal(t, "initial_token", token)

	mgr.Stop()
}

func TestManager_Start_RefreshError(t *testing.T) {
	refreshFn := func(ctx context.Context) (string, time.Duration, error) {
		return "", 0, errors.New("refresh failed")
	}

	mgr := NewManager(Config{
		RefreshFunc:     refreshFn,
		RefreshInterval: 100 * time.Millisecond,
	})

	ctx := context.Background()

	// Start should fail if initial refresh fails
	err := mgr.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "refresh failed")
}

func TestManager_RefreshLoop(t *testing.T) {
	refreshCount := 0
	refreshFn := func(ctx context.Context) (string, time.Duration, error) {
		refreshCount++
		return "token_" + string(rune('0'+refreshCount)), 2 * time.Hour, nil
	}

	mgr := NewManager(Config{
		RefreshFunc:     refreshFn,
		RefreshInterval: 50 * time.Millisecond,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := mgr.Start(ctx)
	require.NoError(t, err)

	// Wait for a few refresh cycles
	time.Sleep(150 * time.Millisecond)

	mgr.Stop()

	// Should have refreshed multiple times
	assert.GreaterOrEqual(t, refreshCount, 2)
}

func TestErrors(t *testing.T) {
	// Test error messages
	assert.Equal(t, "token expired or not available", ErrTokenExpired.Error())
	assert.Equal(t, "no refresh function configured", ErrNoRefreshFunc.Error())
}
