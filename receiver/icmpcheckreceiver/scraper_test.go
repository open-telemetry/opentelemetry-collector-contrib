// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package icmpcheckreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/icmpcheckreceiver/internal/metadata"
)

type mockPinger struct {
	pingResult
}

func (p *mockPinger) IPString() string { return p.targetIP }

func (p *mockPinger) HostName() string { return p.targetHost }

func (p *mockPinger) Stats() *pingStats { return p.stats }

func (p *mockPinger) Run() error { return p.err }

func TestNewScraper(t *testing.T) {
	cfg := &Config{
		Targets: []PingTarget{
			{Host: "example.com"},
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
	settings := receivertest.NewNopSettings(metadata.Type)

	scraper := newScraper(cfg, settings)

	assert.NotNil(t, scraper)
	assert.Equal(t, cfg, scraper.cfg)
	assert.NotNil(t, scraper.mb)
	assert.Equal(t, settings.TelemetrySettings, scraper.settings)
}

func TestIcmpCheckScraperStart(t *testing.T) {
	tests := []struct {
		name           string
		targets        []PingTarget
		expectedCounts []int
	}{
		{
			name: "apply defaults to empty targets",
			targets: []PingTarget{
				{Host: "example.com"},
				{Host: "google.com"},
			},
			expectedCounts: []int{DefaultPingCount, DefaultPingCount},
		},
		{
			name: "keep existing values",
			targets: []PingTarget{
				{Host: "example.com", PingCount: 5, PingTimeout: 2 * time.Second, PingInterval: 5 * time.Second},
			},
			expectedCounts: []int{5},
		},
		{
			name: "mix of default and custom values",
			targets: []PingTarget{
				{Host: "example.com", PingCount: 10},
				{Host: "google.com"},
			},
			expectedCounts: []int{10, DefaultPingCount},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Targets:              tt.targets,
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			}
			settings := receivertest.NewNopSettings(metadata.Type)
			scraper := newScraper(cfg, settings)

			err := scraper.start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err)

			for i, target := range scraper.cfg.Targets {
				assert.Equal(t, tt.expectedCounts[i], target.PingCount)
				if tt.targets[i].PingTimeout == 0 {
					assert.Equal(t, DefaultPingTimeout, target.PingTimeout)
				}
				if tt.targets[i].PingInterval == 0 {
					assert.Equal(t, DefaultPingInterval, target.PingInterval)
				}
			}
			assert.NotNil(t, scraper.pingerFactory)
		})
	}
}

func TestPingSuccess(t *testing.T) {
	pinger := &mockPinger{
		pingResult{
			targetHost: "test-host",
			targetIP:   "127.0.0.1",
			stats:      &pingStats{},
		},
	}
	results := make(chan pingResult, 1)

	go ping(pinger, results)

	select {
	case result := <-results:
		assert.Equal(t, "test-host", result.targetHost)
		assert.Equal(t, "127.0.0.1", result.targetIP)
		assert.NoError(t, result.err)
		assert.NotNil(t, result.stats)
	case <-time.After(5 * time.Second):
		t.Fatal("ping took too long")
	}
}

func TestPingError(t *testing.T) {
	pinger := &mockPinger{
		pingResult{
			targetHost: "test-host",
			err:        assert.AnError,
		},
	}
	results := make(chan pingResult, 1)

	go ping(pinger, results)

	select {
	case result := <-results:
		assert.Equal(t, "test-host", result.targetHost)
		assert.Empty(t, result.targetIP)
		assert.Nil(t, result.stats)
		assert.Error(t, result.err)
	case <-time.After(5 * time.Second):
		t.Fatal("ping took too long")
	}
}

func TestAddMetrics(t *testing.T) {
	testCases := []struct {
		name            string
		given           pingResult
		expected        map[string]any
		customizeConfig func(cfg *Config)
	}{
		{
			name: "all metrics enabled",
			given: pingResult{
				targetHost: "example.com",
				targetIP:   "192.168.1.1",
				stats: &pingStats{
					minRtt:    10 * time.Millisecond,
					maxRtt:    20 * time.Millisecond,
					avgRtt:    15 * time.Millisecond,
					stdDevRtt: 5 * time.Millisecond,
					lossRatio: 0.1,
				},
				err: nil,
			},
			expected: map[string]any{
				"ping.rtt.min":    int64(10),
				"ping.rtt.max":    int64(20),
				"ping.rtt.avg":    int64(15),
				"ping.rtt.stddev": int64(5),
				"ping.loss.ratio": 0.1,
			},
		},
		{
			name: "error case",
			given: pingResult{
				targetHost: "example.com",
				err:        assert.AnError,
			},
		},
		{
			name: "some metrics disabled",
			given: pingResult{
				targetHost: "example.com",
				targetIP:   "192.168.1.1",
				stats: &pingStats{
					minRtt:    10 * time.Millisecond,
					maxRtt:    20 * time.Millisecond,
					avgRtt:    15 * time.Millisecond,
					stdDevRtt: 5 * time.Millisecond,
					lossRatio: 0.1,
				},
				err: nil,
			},
			expected: map[string]any{
				"ping.rtt.min":    int64(10),
				"ping.rtt.max":    int64(20),
				"ping.rtt.stddev": int64(5),
			},
			customizeConfig: func(cfg *Config) {
				cfg.Metrics.PingRttAvg.Enabled = false
				cfg.Metrics.PingLossRatio.Enabled = false
			},
		},
		{
			name: "all metrics disabled",
			given: pingResult{
				targetHost: "example.com",
				targetIP:   "192.168.1.1",
				stats: &pingStats{
					minRtt:    10 * time.Millisecond,
					maxRtt:    20 * time.Millisecond,
					avgRtt:    15 * time.Millisecond,
					lossRatio: 0.1,
				},
				err: nil,
			},
			expected: map[string]any{},
			customizeConfig: func(cfg *Config) {
				cfg.Metrics.PingRttMin.Enabled = false
				cfg.Metrics.PingRttMax.Enabled = false
				cfg.Metrics.PingRttAvg.Enabled = false
				cfg.Metrics.PingLossRatio.Enabled = false
				cfg.Metrics.PingRttStddev.Enabled = false
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			}
			if tc.customizeConfig != nil {
				tc.customizeConfig(cfg)
			}
			settings := receivertest.NewNopSettings(metadata.Type)
			mb := metadata.NewMetricsBuilder(cfg.MetricsBuilderConfig, settings)

			logger := zap.NewNop()
			addMetrics(tc.given, mb, logger)
			metrics := mb.Emit()

			if len(tc.expected) == 0 {
				assert.Equal(t, 0, metrics.DataPointCount())
				return
			}

			assert.Positive(t, metrics.DataPointCount())

			rm := metrics.ResourceMetrics().At(0)
			ilm := rm.ScopeMetrics().At(0)

			assert.Equal(t, len(tc.expected), ilm.Metrics().Len())

			for i := 0; i < ilm.Metrics().Len(); i++ {
				metric := ilm.Metrics().At(i)
				name := metric.Name()

				expectedValue, exists := tc.expected[name]
				assert.True(t, exists, "Unexpected metric name: %s", name)

				dp := metric.Gauge().DataPoints().At(0)

				switch dp.ValueType() {
				case pmetric.NumberDataPointValueTypeInt:
					assert.Equal(t, expectedValue, dp.IntValue())
				case pmetric.NumberDataPointValueTypeDouble:
					assert.Equal(t, expectedValue, dp.DoubleValue())
				default:
					t.Errorf("Unexpected data point value type for metric %s", name)
				}
			}
		})
	}
}

func TestIcmpCheckScraperScrape(t *testing.T) {
	cfg := &Config{
		Targets: []PingTarget{
			{Host: "127.0.0.1", PingCount: 1, PingTimeout: 1 * time.Second, PingInterval: 1 * time.Second},
			{Host: "example.com"},
		},
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
	settings := receivertest.NewNopSettings(metadata.Type)
	scraper := newScraper(cfg, settings)
	scraper.pingerFactory = func(target PingTarget) (pinger, error) {
		return &mockPinger{
			pingResult{
				targetHost: target.Host,
				targetIP:   "212.133.0.1",
				stats: &pingStats{
					minRtt:    10 * time.Millisecond,
					maxRtt:    20 * time.Millisecond,
					avgRtt:    15 * time.Millisecond,
					lossRatio: 0.0,
				},
				err: nil,
			},
		}, nil
	}

	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	metrics, err := scraper.scrape(t.Context())
	require.NoError(t, err)
	assert.NotNil(t, metrics)

	// Verify that metrics were generated
	assert.Positive(t, metrics.DataPointCount())
	assert.Equal(t, 2, metrics.ResourceMetrics().Len())

	for idx := range cfg.Targets {
		rm := metrics.ResourceMetrics().At(idx)

		_, hasNameAttr := rm.Resource().Attributes().Get("net.peer.name")
		_, hasIPAttr := rm.Resource().Attributes().Get("net.peer.name")
		assert.True(t, hasNameAttr)
		assert.True(t, hasIPAttr)

		ilm := rm.ScopeMetrics().At(0)
		assert.Equal(t, 5, ilm.Metrics().Len())
	}
}
