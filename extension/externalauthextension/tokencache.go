package externalauthextension

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type TokenInfo struct {
	Valid    bool
	LastSeen time.Time
}

type TokenCache struct {
	mu    sync.RWMutex
	cache map[string]TokenInfo
}

func newTokenCache() *TokenCache {
	return &TokenCache{
		cache: make(map[string]TokenInfo),
	}
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func (tc *TokenCache) addToken(token string, valid bool) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	hashedToken := hashToken(token)
	tokenInfo := TokenInfo{
		Valid:    valid,
		LastSeen: time.Now(),
	}

	tc.cache[hashedToken] = tokenInfo
}

func (tc *TokenCache) validateToken(token string) bool {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	hashedToken := hashToken(token)
	tokenInfo, exists := tc.cache[hashedToken]
	if !exists {
		return false
	}

	tokenInfo.Valid = true
	tokenInfo.LastSeen = time.Now()
	tc.cache[hashedToken] = tokenInfo
	return true
}

func (tc *TokenCache) invalidateToken(token string) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	hashedToken := hashToken(token)
	tokenInfo, exists := tc.cache[hashedToken]
	if exists {
		tokenInfo.Valid = false
		tokenInfo.LastSeen = time.Now()
		tc.cache[hashedToken] = tokenInfo
	}
}

func (tc *TokenCache) isTokenExpired(token string, durationStr string) bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	duration, _ := time.ParseDuration(durationStr)

	hashedToken := hashToken(token)
	tokenInfo, exists := tc.cache[hashedToken]
	if !exists {
		return true
	}

	return time.Since(tokenInfo.LastSeen) > duration
}

func (tc *TokenCache) tokenExists(token string) bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	hashedToken := hashToken(token)
	_, exists := tc.cache[hashedToken]
	return exists
}

func (tc *TokenCache) isTokenValid(token string) bool {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	hashedToken := hashToken(token)
	tokenInfo, exists := tc.cache[hashedToken]
	if exists {
		return tokenInfo.Valid
	}
	return false
}
func (tc *TokenCache) setTokenExpiry(token string, expiryTime time.Time) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	hashedToken := hashToken(token)
	tokenInfo, exists := tc.cache[hashedToken]
	if exists {
		tokenInfo.LastSeen = expiryTime
		tc.cache[hashedToken] = tokenInfo
	}
}
