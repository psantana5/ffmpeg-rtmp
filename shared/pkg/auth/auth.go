package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrTokenExpired = errors.New("token expired")
)

// TokenManager manages API authentication tokens
type TokenManager struct {
	tokens map[string]*TokenInfo
	mu     sync.RWMutex
}

// TokenInfo contains token metadata
type TokenInfo struct {
	Hash      string
	NodeID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// NewTokenManager creates a new token manager
func NewTokenManager() *TokenManager {
	return &TokenManager{
		tokens: make(map[string]*TokenInfo),
	}
}

// GenerateToken generates a new authentication token
func (tm *TokenManager) GenerateToken(nodeID string, duration time.Duration) (string, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// Hash token for storage
	hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash token: %w", err)
	}

	// Store token info
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.tokens[nodeID] = &TokenInfo{
		Hash:      string(hash),
		NodeID:    nodeID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(duration),
	}

	return token, nil
}

// ValidateToken validates an authentication token
func (tm *TokenManager) ValidateToken(nodeID, token string) error {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tokenInfo, ok := tm.tokens[nodeID]
	if !ok {
		return ErrInvalidToken
	}

	// Check expiration
	if time.Now().After(tokenInfo.ExpiresAt) {
		return ErrTokenExpired
	}

	// Validate token hash
	if err := bcrypt.CompareHashAndPassword([]byte(tokenInfo.Hash), []byte(token)); err != nil {
		return ErrInvalidToken
	}

	return nil
}

// RevokeToken revokes a token for a node
func (tm *TokenManager) RevokeToken(nodeID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	delete(tm.tokens, nodeID)
}

// CleanupExpiredTokens removes expired tokens
func (tm *TokenManager) CleanupExpiredTokens() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	for nodeID, tokenInfo := range tm.tokens {
		if now.After(tokenInfo.ExpiresAt) {
			delete(tm.tokens, nodeID)
		}
	}
}

// APIKeyManager manages API keys for authentication
type APIKeyManager struct {
	keys map[string]string // key -> description
	mu   sync.RWMutex
}

// NewAPIKeyManager creates a new API key manager
func NewAPIKeyManager() *APIKeyManager {
	return &APIKeyManager{
		keys: make(map[string]string),
	}
}

// GenerateAPIKey generates a new API key
func (akm *APIKeyManager) GenerateAPIKey(description string) (string, error) {
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}

	apiKey := base64.URLEncoding.EncodeToString(keyBytes)

	akm.mu.Lock()
	defer akm.mu.Unlock()

	akm.keys[apiKey] = description
	return apiKey, nil
}

// ValidateAPIKey validates an API key
func (akm *APIKeyManager) ValidateAPIKey(apiKey string) bool {
	akm.mu.RLock()
	defer akm.mu.RUnlock()

	_, ok := akm.keys[apiKey]
	return ok
}

// RevokeAPIKey revokes an API key
func (akm *APIKeyManager) RevokeAPIKey(apiKey string) {
	akm.mu.Lock()
	defer akm.mu.Unlock()

	delete(akm.keys, apiKey)
}

// ListAPIKeys returns all API keys with their descriptions
func (akm *APIKeyManager) ListAPIKeys() map[string]string {
	akm.mu.RLock()
	defer akm.mu.RUnlock()

	keys := make(map[string]string, len(akm.keys))
	for k, v := range akm.keys {
		keys[k] = v
	}
	return keys
}

// SecureCompare performs constant-time comparison
func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
