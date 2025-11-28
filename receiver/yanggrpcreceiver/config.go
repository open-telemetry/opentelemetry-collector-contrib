// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package yanggrpcreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/yanggrpcreceiver"

import (
	"time"

	"go.opentelemetry.io/collector/config/configgrpc"
)

// SecurityConfig contains security hardening options
type SecurityConfig struct {
	// RateLimiting contains rate limiting configuration
	RateLimiting RateLimitingConfig `mapstructure:"rate_limiting"`

	// AllowedClients contains client IP allowlist configuration
	AllowedClients []string `mapstructure:"allowed_clients"`

	// MaxConnections is the maximum number of concurrent connections
	MaxConnections int `mapstructure:"max_connections"`

	// ConnectionTimeout is the maximum time to wait for new connections
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout"`

	// EnableMetrics enables security-related metrics collection
	EnableMetrics bool `mapstructure:"enable_metrics"`
}

// RateLimitingConfig contains rate limiting configuration
type RateLimitingConfig struct {
	// Enabled indicates whether rate limiting should be enabled
	Enabled bool `mapstructure:"enabled"`

	// RequestsPerSecond is the maximum number of requests per second per client
	RequestsPerSecond float64 `mapstructure:"requests_per_second"`

	// BurstSize is the maximum burst size for rate limiting
	BurstSize int `mapstructure:"burst_size"`

	// CleanupInterval is how often to clean up rate limiter entries
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

// YANGConfig contains YANG parser configuration
type YANGConfig struct {
	// EnableRFCParser enables the RFC 6020/7950 compliant YANG parser
	EnableRFCParser bool `mapstructure:"enable_rfc_parser"`

	// CacheModules enables caching of discovered YANG modules
	CacheModules bool `mapstructure:"cache_modules"`

	// MaxModules is the maximum number of YANG modules to cache
	MaxModules int `mapstructure:"max_modules"`
}

// Config defines configuration for yanggrpc receiver.
type Config struct {
	configgrpc.ServerConfig `mapstructure:",squash"`

	// YANG contains YANG parser configuration
	YANG YANGConfig `mapstructure:"yang"`

	// Security contains security hardening configuration
	Security SecurityConfig `mapstructure:"security"`
}

// Validate checks the receiver configuration is valid
func (c *Config) Validate() error {
	return c.ServerConfig.Validate()
}
