// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package dns provides a DNS-based lookup source.
package dns // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/internal/source/dns"

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

const sourceType = "dns"

type RecordType string

const (
	// Only PTR for now.
	RecordTypePTR RecordType = "PTR"
)

type Config struct {
	// RecordType specifies the DNS record type to look up.
	RecordType RecordType `mapstructure:"record_type"`

	// Timeout is the maximum time to wait for a DNS query.
	// Default: 1 second
	Timeout time.Duration `mapstructure:"timeout"`

	// Server is the DNS server to use (e.g., "8.8.8.8:53").
	// If empty, uses the system resolver.
	Server string `mapstructure:"server"`

	// Cache configures caching for DNS lookups.
	// Enabled by default.
	// Disabling is not recommended due to potential performance impact.
	Cache lookupsource.CacheConfig `mapstructure:"cache"`
}

func (c *Config) Validate() error {
	switch c.RecordType {
	case "", RecordTypePTR:
		// Valid
	default:
		return fmt.Errorf("invalid record_type %q, only PTR is currently supported", c.RecordType)
	}

	if c.Timeout <= 0 {
		return errors.New("timeout must be greater than 0")
	}

	if err := c.Cache.Validate(); err != nil {
		return err
	}

	return nil
}

func NewFactory() lookupsource.SourceFactory {
	return lookupsource.NewSourceFactory(
		sourceType,
		createDefaultConfig,
		createSource,
	)
}

func createDefaultConfig() lookupsource.SourceConfig {
	return &Config{
		RecordType: RecordTypePTR,
		Timeout:    1 * time.Second,
		Cache: lookupsource.CacheConfig{
			Enabled:     true,
			Size:        10000,
			TTL:         5 * time.Minute,
			NegativeTTL: 1 * time.Minute,
		},
	}
}

func createSource(
	_ context.Context,
	_ lookupsource.CreateSettings,
	cfg lookupsource.SourceConfig,
) (lookupsource.Source, error) {
	dnsCfg := cfg.(*Config)

	recordType := dnsCfg.RecordType
	if recordType == "" {
		recordType = RecordTypePTR
	}

	timeout := dnsCfg.Timeout

	var resolver *net.Resolver
	if dnsCfg.Server != "" {
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
				d := net.Dialer{}
				return d.DialContext(ctx, network, dnsCfg.Server)
			},
		}
	} else {
		resolver = net.DefaultResolver
	}

	s := &dnsSource{
		recordType: recordType,
		timeout:    timeout,
		resolver:   resolver,
	}

	// Create the lookup function, optionally wrapped with cache
	lookupFn := s.lookup
	if dnsCfg.Cache.Enabled {
		cache := lookupsource.NewCache(dnsCfg.Cache)
		lookupFn = lookupsource.WrapWithCache(cache, lookupFn)
	}

	return lookupsource.NewSource(
		lookupFn,
		func() string { return sourceType },
		nil, // no start needed
		nil, // no shutdown needed
	), nil
}

type dnsSource struct {
	recordType RecordType
	timeout    time.Duration
	resolver   *net.Resolver
}

func (s *dnsSource) lookup(ctx context.Context, key string) (any, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	// Currently only PTR is supported
	return s.lookupPTR(ctx, key)
}

// lookupPTR performs reverse DNS lookup (IP -> hostname).
func (s *dnsSource) lookupPTR(ctx context.Context, ip string) (any, bool, error) {
	names, err := s.resolver.LookupAddr(ctx, ip)
	if err != nil {
		// DNS errors for non-existent records should return not found, not error
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && (dnsErr.IsNotFound || dnsErr.IsTemporary) {
			return nil, false, nil
		}
		return nil, false, err
	}

	if len(names) == 0 {
		return nil, false, nil
	}

	// Return the first hostname, trimming trailing dot
	return strings.TrimSuffix(names[0], "."), true, nil
}
