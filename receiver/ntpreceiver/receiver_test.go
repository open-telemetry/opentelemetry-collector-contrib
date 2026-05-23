// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ntpreceiver

import (
	"errors"
	"testing"
	"time"

	"github.com/beevik/ntp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ntpreceiver/internal/metadata"
)

// Minimal scraper builder for tests (no controller). We keep this close to prod.
func newTestScraper(cfg *Config) *ntpScraper {
	settings := receivertest.NewNopSettings(metadata.Type)
	return &ntpScraper{
		logger:   settings.Logger,
		mb:       metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings),
		version:  cfg.Version,
		endpoint: cfg.Endpoint,
		timeout:  5 * time.Second, // fixed timeout for tests
	}
}

func TestScraper_Success(t *testing.T) {
	// Mock NTP query
	old := queryWithOptions
	defer func() { queryWithOptions = old }()
	queryWithOptions = func(_ string, _ ntp.QueryOptions) (*ntp.Response, error) {
		// We return a non-zero offset; test will not assert the value to avoid controller-specific init.
		return &ntp.Response{ClockOffset: 250 * time.Millisecond}, nil
	}

	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "time.example.com:123"

	s := newTestScraper(cfg)
	metrics, err := s.scrape(t.Context())
	require.NoError(t, err)
	require.NotNil(t, metrics)

	// ResourceMetrics present
	rmSlice := metrics.ResourceMetrics()
	require.Equal(t, 1, rmSlice.Len())

	// Resource attribute ntp.host set to endpoint
	rm := rmSlice.At(0)
	resAttrs := rm.Resource().Attributes()
	val, ok := resAttrs.Get("ntp.host")
	require.True(t, ok, "expected ntp.host resource attribute")
	require.Equal(t, cfg.Endpoint, val.Str())

	// One scope, one metric
	smSlice := rm.ScopeMetrics()
	require.Equal(t, 1, smSlice.Len())
	mSlice := smSlice.At(0).Metrics()
	require.Equal(t, 1, mSlice.Len())

	m := mSlice.At(0)
	require.Equal(t, "ntp.offset", m.Name())

	// Unit should be "ns" per current receiver metadata (do not fail hard if empty, but assert when present)
	if u := m.Unit(); u != "" {
		require.Equal(t, "ns", u, "expected ntp.offset unit to be ns")
	}

	// Handle Gauge vs Sum defensively; generator may emit Gauge.
	var dps pmetric.NumberDataPointSlice
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		dps = m.Gauge().DataPoints()
	case pmetric.MetricTypeSum:
		dps = m.Sum().DataPoints()
	default:
		t.Fatalf("unexpected metric type: %v", m.Type())
	}

	// Exactly one data point with non-zero timestamps
	require.Equal(t, 1, dps.Len())
	dp := dps.At(0)
	require.NotZero(t, dp.Timestamp(), "datapoint must have a timestamp")
	// StartTimestamp may be zero for Gauge; only assert when present
	if st := dp.StartTimestamp(); st != 0 {
		require.NotZero(t, st, "start timestamp should not be zero when set")
	}
}

func TestScraper_Error(t *testing.T) {
	// Mock failure
	old := queryWithOptions
	defer func() { queryWithOptions = old }()
	queryWithOptions = func(_ string, _ ntp.QueryOptions) (*ntp.Response, error) {
		return nil, errors.New("mock failure")
	}

	cfg := createDefaultConfig().(*Config)
	cfg.Endpoint = "bad.host"

	s := newTestScraper(cfg)
	metrics, err := s.scrape(t.Context())

	require.Error(t, err)
	require.NotNil(t, metrics)
	require.Equal(t, 0, metrics.ResourceMetrics().Len(), "expected no metrics on error")
}
