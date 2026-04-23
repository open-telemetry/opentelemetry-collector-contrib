// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver/internal"

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
)

// SecurityManager manages security features for the gRPC receiver
type SecurityManager struct {
	allowedClients []string
	rateLimiter    *RateLimiter
	logger         *zap.Logger
}

// RateLimiter implements per-client rate limiting
type RateLimiter struct {
	limiters        map[string]*rate.Limiter
	mu              sync.RWMutex
	requestsPerSec  rate.Limit
	burstSize       int
	cleanupInterval time.Duration
	cleanupTicker   *time.Ticker
	done            chan bool
}

// NewSecurityManager creates a new SecurityManager
func NewSecurityManager(allowedClients []string, logger *zap.Logger, ratelimitingEnabled bool, requestsPerSecond float64, burstSize int, cleanupInterval time.Duration) *SecurityManager {
	sm := &SecurityManager{
		allowedClients: allowedClients,
		logger:         logger,
	}

	// Initialize rate limiter if enabled
	if ratelimitingEnabled {
		sm.rateLimiter = newRateLimiter(
			rate.Limit(requestsPerSecond),
			burstSize,
			cleanupInterval,
		)
	}

	return sm
}

// newRateLimiter creates a new RateLimiter
func newRateLimiter(requestsPerSec rate.Limit, burstSize int, cleanupInterval time.Duration) *RateLimiter {
	rl := &RateLimiter{
		limiters:        make(map[string]*rate.Limiter),
		requestsPerSec:  requestsPerSec,
		burstSize:       burstSize,
		cleanupInterval: cleanupInterval,
		done:            make(chan bool),
	}

	// Start cleanup goroutine
	rl.cleanupTicker = time.NewTicker(cleanupInterval)
	go rl.cleanup()

	return rl
}

// Allow checks if the request from the given IP is allowed
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists := rl.limiters[ip]
	if !exists {
		limiter = rate.NewLimiter(rl.requestsPerSec, rl.burstSize)
		rl.limiters[ip] = limiter
	}

	return limiter.Allow()
}

// cleanup removes unused rate limiters
func (rl *RateLimiter) cleanup() {
	for {
		select {
		case <-rl.cleanupTicker.C:
			rl.mu.Lock()
			// Remove limiters that haven't been used recently
			// For simplicity, we remove all limiters periodically
			// In production, you might want to track last access times
			rl.limiters = make(map[string]*rate.Limiter)
			rl.mu.Unlock()
		case <-rl.done:
			rl.cleanupTicker.Stop()
			return
		}
	}
}

// Stop stops the rate limiter cleanup
func (rl *RateLimiter) Stop() {
	close(rl.done)
}

// getClientAuthType converts string auth type to tls.ClientAuthType
func (*SecurityManager) getClientAuthType(authType string) (tls.ClientAuthType, error) {
	authTypeMap := map[string]tls.ClientAuthType{
		"NoClientCert":               tls.NoClientCert,
		"RequestClientCert":          tls.RequestClientCert,
		"RequireAnyClientCert":       tls.RequireAnyClientCert,
		"VerifyClientCertIfGiven":    tls.VerifyClientCertIfGiven,
		"RequireAndVerifyClientCert": tls.RequireAndVerifyClientCert,
	}

	if authType, exists := authTypeMap[authType]; exists {
		return authType, nil
	}

	return tls.NoClientCert, fmt.Errorf("invalid client auth type: %s", authType)
}

// CreateSecurityInterceptor creates a gRPC interceptor for security enforcement
func (sm *SecurityManager) CreateSecurityInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		// Get client IP
		clientIP, err := sm.getClientIP(ctx)
		if err != nil {
			sm.logger.Warn("Failed to get client IP", zap.Error(err))
			clientIP = "unknown"
		}

		// Check IP allowlist if configured
		if len(sm.allowedClients) > 0 && !sm.isIPAllowed(clientIP) {
			sm.logger.Warn("Client IP not in allowlist", zap.String("client_ip", clientIP))
			return nil, errors.New("client IP not allowed")
		}

		// Apply rate limiting if enabled
		if sm.rateLimiter != nil && !sm.rateLimiter.Allow(clientIP) {
			sm.logger.Warn("Rate limit exceeded", zap.String("client_ip", clientIP))
			return nil, errors.New("rate limit exceeded")
		}

		sm.logger.Debug("Security check passed",
			zap.String("client_ip", clientIP),
			zap.String("method", info.FullMethod))

		return handler(ctx, req)
	}
}

// getClientIP extracts the client IP from the gRPC context
func (*SecurityManager) getClientIP(ctx context.Context) (string, error) {
	peer, ok := peer.FromContext(ctx)
	if !ok {
		return "", errors.New("no peer information in context")
	}

	if peer.Addr == nil {
		return "", errors.New("no address in peer information")
	}

	// Extract IP from address
	host, _, err := net.SplitHostPort(peer.Addr.String())
	if err != nil {
		return "", fmt.Errorf("failed to parse peer address: %w", err)
	}

	return host, nil
}

// isIPAllowed checks if the given IP is in the allowlist
func (sm *SecurityManager) isIPAllowed(clientIP string) bool {
	for _, allowedIP := range sm.allowedClients {
		// Support CIDR notation
		if strings.Contains(allowedIP, "/") {
			_, cidr, err := net.ParseCIDR(allowedIP)
			if err != nil {
				sm.logger.Warn("Invalid CIDR in allowed_clients", zap.String("cidr", allowedIP))
				continue
			}
			ip := net.ParseIP(clientIP)
			if ip != nil && cidr.Contains(ip) {
				return true
			}
			// Direct IP match
		} else if clientIP == allowedIP {
			return true
		}
	}
	return false
}

// Shutdown stops the security manager
func (sm *SecurityManager) Shutdown() {
	if sm.rateLimiter != nil {
		sm.rateLimiter.Stop()
	}
}
