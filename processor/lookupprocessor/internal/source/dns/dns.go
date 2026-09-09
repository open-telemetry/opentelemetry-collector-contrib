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
	// RecordTypePTR performs reverse DNS lookup (IP address to hostname).
	RecordTypePTR RecordType = "PTR"
	// RecordTypeA performs forward IPv4 DNS lookup (hostname to IPv4 address).
	RecordTypeA RecordType = "A"
	// RecordTypeAAAA performs forward IPv6 DNS lookup (hostname to IPv6 address).
	RecordTypeAAAA RecordType = "AAAA"
)

type Config struct {
	// RecordType specifies the DNS record type to look up: "PTR", "A", or "AAAA".
	// - PTR: reverse lookup, resolves IP address to hostname(s)
	// - A: forward IPv4 lookup, resolves hostname to IPv4 address(es)
	// - AAAA: forward IPv6 lookup, resolves hostname to IPv6 address(es)
	// Default: "PTR"
	RecordType RecordType `mapstructure:"record_type"`

	// Timeout is the maximum time to wait for a DNS query.
	// Default: 1 second
	Timeout time.Duration `mapstructure:"timeout"`

	// Server is the DNS server to use (e.g., "8.8.8.8:53").
	// If empty, uses the system resolver.
	Server string `mapstructure:"server"`

	// MultipleResults specifies whether to return all DNS results or just the first.
	// If true, multiple results are returned as a comma-separated string.
	// Default: false
	MultipleResults bool `mapstructure:"multiple_results"`

	// Cache configures caching for DNS lookups.
	// Enabled by default.
	// Disabling is not recommended due to potential performance impact.
	Cache lookupsource.CacheConfig `mapstructure:"cache"`
}

func (c *Config) Validate() error {
	switch c.RecordType {
	case "", RecordTypePTR, RecordTypeA, RecordTypeAAAA:
		// Valid
	default:
		return fmt.Errorf("invalid record_type %q, must be one of: PTR, A, AAAA", c.RecordType)
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
		recordType:      recordType,
		timeout:         timeout,
		resolver:        resolver,
		multipleResults: dnsCfg.MultipleResults,
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
	recordType      RecordType
	timeout         time.Duration
	resolver        *net.Resolver
	multipleResults bool
}

func (s *dnsSource) lookup(ctx context.Context, key string) (any, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	switch s.recordType {
	case RecordTypeA, RecordTypeAAAA:
		return s.lookupForward(ctx, key)
	default:
		return s.lookupPTR(ctx, key)
	}
}

// lookupForward performs forward DNS lookup (hostname -> IP addresses).
func (s *dnsSource) lookupForward(ctx context.Context, hostname string) (any, bool, error) {
	addrs, err := s.resolver.LookupIPAddr(ctx, hostname)
	if err != nil {
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) && (dnsErr.IsNotFound || dnsErr.IsTemporary) {
			return nil, false, nil
		}
		return nil, false, err
	}

	if len(addrs) == 0 {
		return nil, false, nil
	}

	var ips []string
	for _, addr := range addrs {
		if s.recordType == RecordTypeA && addr.IP.To4() != nil {
			// IPv4 address for A record
			ips = append(ips, addr.IP.String())
		} else if s.recordType == RecordTypeAAAA && addr.IP.To4() == nil && addr.IP.To16() != nil {
			// IPv6 address for AAAA record
			ips = append(ips, addr.IP.String())
		}
	}

	if len(ips) == 0 {
		return nil, false, nil
	}

	if s.multipleResults {
		return strings.Join(ips, ","), true, nil
	}
	return ips[0], true, nil
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

	// Trim trailing dots from PTR records
	for i := range names {
		names[i] = strings.TrimSuffix(names[i], ".")
	}

	if s.multipleResults {
		return strings.Join(names, ","), true, nil
	}
	return names[0], true, nil
}
