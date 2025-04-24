// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dnslookupprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/dnslookupprocessor"
import (
	fmt "fmt"
	"strings"

	"go.opentelemetry.io/collector/component"
)

type ContextID string

const (
	resource ContextID = "resource"
	record   ContextID = "record"
)

func (c *ContextID) UnmarshalText(text []byte) error {
	str := ContextID(strings.ToLower(string(text)))
	switch str {
	case resource, record:
		*c = str
		return nil
	default:
		return fmt.Errorf("unknown context %s, available values: %s, %s", str, resource, record)
	}
}

// Config holds the configuration for the DnsLookup processor.
type Config struct {
	// Resolve contains configuration for forward DNS lookups (hostname to IP).
	Resolve LookupConfig `mapstructure:"resolve"`

	// Reverse contains configuration for reverse DNS lookups (IP to hostname).
	Reverse LookupConfig `mapstructure:"reverse"`

	// HitCacheSize defines the maximum number of successful resolutions to cache.
	// Set to 0 to disable caching.
	HitCacheSize int `mapstructure:"hit_cache_size"`

	// HitCacheTTL defines the time-to-live (in seconds) for successful resolution cache entries.
	HitCacheTTL int `mapstructure:"hit_cache_ttl"`

	// MissCacheSize defines the maximum number of failed resolutions to cache.
	// Set to 0 to disable caching.
	MissCacheSize int `mapstructure:"miss_cache_size"`

	// MissCacheTTL defines the time-to-live (in seconds) for failed resolution cache entries.
	MissCacheTTL int `mapstructure:"miss_cache_ttl"`

	// MaxRetries defines the maximum number of retry attempts for DNS lookups.
	MaxRetries int `mapstructure:"max_retries"`

	// Timeout defines the timeout duration (in seconds) for individual DNS lookups through DNS servers.
	Timeout float64 `mapstructure:"timeout"`

	// Hostfiles specifies the path to custom host files.
	Hostfiles []string `mapstructure:"hostfiles"`

	// Nameservers specifies the addresses of custom DNS servers.
	Nameservers []string `mapstructure:"nameservers"`

	// EnableSystemResolver determines whether to use the system's default resolver.
	EnableSystemResolver bool `mapstructure:"enable_system_resolver"`
}

// LookupConfig defines the configuration for forward/reverse DNS resolution.
type LookupConfig struct {
	Enabled bool `mapstructure:"enabled"`

	// Context specifies where to look for attributes (resource or record).
	Context ContextID `mapstructure:"context"`

	// Attributes is a list of attributes to check for hostnames/IP. The first valid hostname/IP is used.
	Attributes []string `mapstructure:"attributes"`

	// ResolvedAttribute is the attribute to store the resolved IP/hostname.
	ResolvedAttribute string `mapstructure:"resolved_attribute"`
}

var _ component.Config = (*Config)(nil)

func (cfg *Config) Validate() error {
	validateLookupConfig := func(lc LookupConfig) error {
		if !lc.Enabled {
			return nil
		}
		if len(lc.Attributes) == 0 {
			return fmt.Errorf("at least one attribute must be specified for DNS resolution")
		}
		if lc.ResolvedAttribute == "" {
			return fmt.Errorf("resolved_attribute must be specified for DNS resolution")
		}
		if lc.Context != resource && lc.Context != record {
			return fmt.Errorf("context must be either 'resource' or 'record', got: %s", lc.Context)
		}
		return nil
	}

	if err := validateLookupConfig(cfg.Resolve); err != nil {
		return fmt.Errorf("resolve configuration: %w", err)
	}

	if err := validateLookupConfig(cfg.Reverse); err != nil {
		return fmt.Errorf("reverse configuration: %w", err)
	}

	if cfg.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got: %f", cfg.Timeout)
	}

	if cfg.MaxRetries < 0 {
		return fmt.Errorf("max_retries must be non-negative, got: %d", cfg.MaxRetries)
	}

	if cfg.HitCacheSize < 0 {
		return fmt.Errorf("hit_cache_size must be non-negative, got: %d", cfg.HitCacheSize)
	}

	if cfg.MissCacheSize < 0 {
		return fmt.Errorf("miss_cache_size must be non-negative, got: %d", cfg.MissCacheSize)
	}

	if cfg.HitCacheTTL <= 0 {
		return fmt.Errorf("hit_cache_ttl must be positive, got: %d", cfg.HitCacheTTL)
	}

	if cfg.MissCacheTTL <= 0 {
		return fmt.Errorf("miss_cache_ttl must be positive, got: %d", cfg.MissCacheTTL)
	}

	if !cfg.EnableSystemResolver && len(cfg.Hostfiles) == 0 && len(cfg.Nameservers) == 0 {
		return fmt.Errorf("at least one of enable_system_resolver, hostfiles, or nameservers must be specified")
	}

	return nil
}
