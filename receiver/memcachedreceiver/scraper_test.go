// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package memcachedreceiver

import (
	"crypto/tls"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetricassert"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver/internal/metadata"
)

func TestScraper(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	scraper := newMemcachedScraper(receivertest.NewNopSettings(metadata.Type), cfg)
	scraper.newClient = func(string, time.Duration, *tls.Config) (client, error) {
		return &fakeClient{}, nil
	}

	actualMetrics, err := scraper.scrape(t.Context())
	require.NoError(t, err)

	expectedFile := filepath.Join("testdata", "scraper", "metrics.assert.yaml")
	// To regenerate: uncomment, run the test once, re-comment.
	// require.NoError(t, pmetricassert.WriteAssertionFile(t, expectedFile, actualMetrics))

	require.NoError(t, pmetricassert.AssertMetrics(expectedFile, actualMetrics))
}

func TestScraperTLSConfigPropagated(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.TLS = configtls.ClientConfig{Insecure: false}
	scraper := newMemcachedScraper(receivertest.NewNopSettings(metadata.Type), cfg)
	scraper.newClient = func(_ string, _ time.Duration, tlsConfig *tls.Config) (client, error) {
		require.NotNil(t, tlsConfig)
		return &fakeClient{}, nil
	}

	_, err := scraper.scrape(t.Context())
	require.NoError(t, err)
}

func TestScraperTLSConfigError(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.TLS = configtls.ClientConfig{
		Config: configtls.Config{CAFile: filepath.Join(t.TempDir(), "missing-ca.crt")},
	}
	scraper := newMemcachedScraper(receivertest.NewNopSettings(metadata.Type), cfg)
	scraper.newClient = func(string, time.Duration, *tls.Config) (client, error) {
		t.Fatal("newClient must not be called when TLS config fails to load")
		return nil, nil
	}

	_, err := scraper.scrape(t.Context())
	require.Error(t, err)
}

func TestCalculateHitRatio(t *testing.T) {
	tests := []struct {
		name     string
		hits     int64
		misses   int64
		expected float64
	}{
		{name: "all hits", hits: 100, misses: 0, expected: 100},
		{name: "all misses", hits: 0, misses: 100, expected: 0},
		{name: "mostly hits", hits: 90, misses: 10, expected: 90},
		{name: "no operations", hits: 0, misses: 0, expected: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, calculateHitRatio(tt.hits, tt.misses))
		})
	}
}
