// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dns

import (
	"net"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/lookupprocessor/lookupsource"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, "dns", factory.Type())
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "empty config is invalid",
			config:  &Config{},
			wantErr: true,
		},
		{
			name: "PTR record type",
			config: &Config{
				RecordType: RecordTypePTR,
				Timeout:    1 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "unsupported record type",
			config: &Config{
				RecordType: "A",
			},
			wantErr: true,
		},
		{
			name: "invalid record type",
			config: &Config{
				RecordType: "INVALID",
			},
			wantErr: true,
		},
		{
			name: "negative timeout",
			config: &Config{
				Timeout: -1 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "zero timeout",
			config: &Config{
				Timeout: 0,
			},
			wantErr: true,
		},
		{
			name: "custom server",
			config: &Config{
				Timeout: 1 * time.Second,
				Server:  "8.8.8.8:53",
			},
			wantErr: false,
		},
		{
			name: "invalid cache size when enabled",
			config: &Config{
				Cache: lookupsource.CacheConfig{
					Enabled: true,
					Size:    0,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateSource(t *testing.T) {
	factory := NewFactory()

	cfg := factory.CreateDefaultConfig()
	source, err := factory.CreateSource(t.Context(), lookupsource.CreateSettings{}, cfg)
	require.NoError(t, err)
	require.NotNil(t, source)
	assert.Equal(t, "dns", source.Type())
}

func TestCreateSourceWithCustomConfig(t *testing.T) {
	factory := NewFactory()

	cfg := &Config{
		RecordType: RecordTypePTR,
		Timeout:    10 * time.Second,
		Server:     "8.8.8.8:53",
		Cache: lookupsource.CacheConfig{
			Enabled: false,
		},
	}

	source, err := factory.CreateSource(t.Context(), lookupsource.CreateSettings{}, cfg)
	require.NoError(t, err)
	require.NotNil(t, source)
}

func TestDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	assert.Equal(t, RecordTypePTR, cfg.RecordType)
	assert.Equal(t, 1*time.Second, cfg.Timeout)
	assert.Empty(t, cfg.Server)
	assert.True(t, cfg.Cache.Enabled)
	assert.Equal(t, 10000, cfg.Cache.Size)
	assert.Equal(t, 5*time.Minute, cfg.Cache.TTL)
	assert.Equal(t, 1*time.Minute, cfg.Cache.NegativeTTL)
}

// Integration tests - these actually perform DNS lookups
// They're skipped in CI but useful for local development

func TestLookupPTR_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	factory := NewFactory()
	cfg := &Config{
		RecordType: RecordTypePTR,
		Timeout:    5 * time.Second,
		Cache: lookupsource.CacheConfig{
			Enabled: false,
		},
	}

	source, err := factory.CreateSource(t.Context(), lookupsource.CreateSettings{}, cfg)
	require.NoError(t, err)

	// Google's public DNS IP should have a PTR record
	val, found, err := source.Lookup(t.Context(), "8.8.8.8")
	require.NoError(t, err)

	if found {
		hostname, ok := val.(string)
		assert.True(t, ok)
		t.Logf("PTR lookup for 8.8.8.8: %s", hostname)
	} else {
		t.Log("PTR lookup for 8.8.8.8 not found (may be network issue)")
	}
}

func TestLookupNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	factory := NewFactory()
	cfg := &Config{
		RecordType: RecordTypePTR,
		Timeout:    2 * time.Second,
		Cache: lookupsource.CacheConfig{
			Enabled: false,
		},
	}

	source, err := factory.CreateSource(t.Context(), lookupsource.CreateSettings{}, cfg)
	require.NoError(t, err)

	// Private IP with no PTR record. The resolver may return not-found
	// (NXDOMAIN) or a timeout depending on the platform and network
	// configuration. Both are acceptable outcomes.
	_, found, err := source.Lookup(t.Context(), "192.168.1.1")
	if err == nil {
		assert.False(t, found)
	} else {
		var dnsErr *net.DNSError
		require.ErrorAs(t, err, &dnsErr)
	}
}

func TestLookupWithCache(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	// Windows timer resolution (~15ms) is too coarse for timing-based
	// cache assertions. Both lookups can measure as 0s when the OS DNS
	// resolver cache makes even the first lookup sub-millisecond.
	if runtime.GOOS == "windows" {
		t.Skip("skipping cache timing test on Windows due to low timer resolution")
	}

	factory := NewFactory()
	cfg := &Config{
		RecordType: RecordTypePTR,
		Timeout:    5 * time.Second,
		Cache: lookupsource.CacheConfig{
			Enabled: true,
			Size:    100,
			TTL:     1 * time.Minute,
		},
	}

	source, err := factory.CreateSource(t.Context(), lookupsource.CreateSettings{}, cfg)
	require.NoError(t, err)

	// First lookup - Google's public DNS
	start := time.Now()
	val1, found1, err := source.Lookup(t.Context(), "8.8.8.8")
	require.NoError(t, err)
	firstDuration := time.Since(start)

	if !found1 {
		t.Skip("DNS lookup failed, skipping cache test")
	}

	// Second lookup should be faster (cached)
	start = time.Now()
	val2, found2, err := source.Lookup(t.Context(), "8.8.8.8")
	require.NoError(t, err)
	secondDuration := time.Since(start)

	assert.True(t, found2)
	assert.Equal(t, val1, val2)

	// Cache hit should be significantly faster than a network lookup
	t.Logf("First lookup: %v, Second lookup (cached): %v", firstDuration, secondDuration)
	assert.Less(t, secondDuration, firstDuration)
}
