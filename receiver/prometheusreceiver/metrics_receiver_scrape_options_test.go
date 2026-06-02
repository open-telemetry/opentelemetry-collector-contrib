// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusreceiver

import (
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/metadata"
)

func TestScrapeOptionsMapping(t *testing.T) {
	mockConsumer := new(consumertest.MetricsSink)
	settings := receivertest.NewNopSettings(metadata.Type)

	t.Run("default", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		r, err := newPrometheusReceiver(settings, cfg, mockConsumer)
		require.NoError(t, err)

		opts := r.initScrapeOptions(prometheusScrapeTestOptions{})
		assert.False(t, opts.ScrapeOnShutdown)
		assert.False(t, opts.DiscoveryReloadOnStartup)
		assert.Zero(t, opts.InitialScrapeOffset)

		require.NoError(t, r.Shutdown(t.Context()))
	})

	t.Run("scrape_on_shutdown", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.ScrapeOnShutdown = true
		r, err := newPrometheusReceiver(settings, cfg, mockConsumer)
		require.NoError(t, err)

		opts := r.initScrapeOptions(prometheusScrapeTestOptions{})
		assert.True(t, opts.ScrapeOnShutdown)

		require.NoError(t, r.Shutdown(t.Context()))
	})

	t.Run("discovery_reload_on_startup", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.DiscoveryReloadOnStartup = true
		r, err := newPrometheusReceiver(settings, cfg, mockConsumer)
		require.NoError(t, err)

		opts := r.initScrapeOptions(prometheusScrapeTestOptions{})
		assert.True(t, opts.DiscoveryReloadOnStartup)

		require.NoError(t, r.Shutdown(t.Context()))
	})

	t.Run("initial_scrape_offset", func(t *testing.T) {
		cfg := createDefaultConfig().(*Config)
		cfg.InitialScrapeOffset = 10 * time.Second
		r, err := newPrometheusReceiver(settings, cfg, mockConsumer)
		require.NoError(t, err)

		opts := r.initScrapeOptions(prometheusScrapeTestOptions{})
		assert.Equal(t, 10*time.Second, opts.InitialScrapeOffset)

		require.NoError(t, r.Shutdown(t.Context()))
	})
}

// TestScrapeOnShutdownFunctional verifies that when ScrapeOnShutdown is enabled,
// the receiver executes a final scrape of all targets immediately before shutting down.
// It does this by setting up a mock Prometheus target with a very long (10-minute) scrape
// interval to prevent background periodic scrapes. It confirms that exactly 1 scrape request
// is served at startup, and that a second (final) request is served when Shutdown is called.
func TestScrapeOnShutdownFunctional(t *testing.T) {
	ctx := t.Context()
	settings := receivertest.NewNopSettings(metadata.Type)

	td := &testData{
		name: "test_shutdown_target",
		pages: []mockPrometheusResponse{
			{
				code: 200,
				data: `
# HELP test_gauge A gauge for testing
# TYPE test_gauge gauge
test_gauge 42
`,
			},
			{
				code: 200,
				data: `
# HELP test_gauge A gauge for testing
# TYPE test_gauge gauge
test_gauge 43
`,
			},
		},
	}

	mp, promCfg, err := setupMockPrometheus(td)
	require.NoError(t, err)
	defer mp.Close()

	for _, sc := range promCfg.ScrapeConfigs {
		sc.ScrapeInterval = model.Duration(10 * time.Minute)
	}

	cfg := &Config{
		PrometheusConfig: promCfg,
		ScrapeOnShutdown: true,
		skipOffsetting:   true,
	}

	sink := new(consumertest.MetricsSink)
	r, err := newPrometheusReceiver(settings, cfg, sink)
	require.NoError(t, err)

	err = r.start(ctx, componenttest.NewNopHost(), prometheusComponentTestOptions{
		discovery: prometheusDiscoveryTestOptions{updatert: 50 * time.Millisecond},
		scrape:    prometheusScrapeTestOptions{discoveryReloadInterval: 50 * time.Millisecond},
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(sink.AllMetrics()) >= 1
	}, 5*time.Second, 10*time.Millisecond)

	metricPath := fmt.Sprintf("/%s/metrics", td.name)
	require.Equal(t, int32(1), mp.accessIndex[metricPath].Load())

	err = r.Shutdown(ctx)
	require.NoError(t, err)

	assert.Equal(t, int32(2), mp.accessIndex[metricPath].Load(), "Mock server should have received exactly 2 requests (1 at startup, 1 on shutdown)")
}

// TestDiscoveryReloadOnStartupFunctional verifies that when DiscoveryReloadOnStartup is enabled,
// target discovery is loaded immediately on start, bypassing a long discovery reload interval.
func TestDiscoveryReloadOnStartupFunctional(t *testing.T) {
	ctx := t.Context()
	settings := receivertest.NewNopSettings(metadata.Type)

	td := &testData{
		name: "test_discovery_target",
		pages: []mockPrometheusResponse{
			{
				code: 200,
				data: `
# HELP test_gauge A gauge for testing
# TYPE test_gauge gauge
test_gauge 42
`,
			},
		},
	}

	mp, promCfg, err := setupMockPrometheus(td)
	require.NoError(t, err)
	defer mp.Close()

	cfg := &Config{
		PrometheusConfig:         promCfg,
		DiscoveryReloadOnStartup: true,
		skipOffsetting:           true,
	}

	sink := new(consumertest.MetricsSink)
	r, err := newPrometheusReceiver(settings, cfg, sink)
	require.NoError(t, err)

	err = r.start(ctx, componenttest.NewNopHost(), prometheusComponentTestOptions{
		discovery: prometheusDiscoveryTestOptions{updatert: 50 * time.Millisecond},
		scrape:    prometheusScrapeTestOptions{discoveryReloadInterval: 10 * time.Minute},
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(sink.AllMetrics()) >= 1
	}, 5*time.Second, 10*time.Millisecond)

	metricPath := fmt.Sprintf("/%s/metrics", td.name)
	assert.Equal(t, int32(1), mp.accessIndex[metricPath].Load(), "Target should be scraped immediately on start due to DiscoveryReloadOnStartup")

	require.NoError(t, r.Shutdown(ctx))
}

// TestInitialScrapeOffsetFunctional verifies that when InitialScrapeOffset is configured,
// the first scrape is delayed by at least the configured duration.
func TestInitialScrapeOffsetFunctional(t *testing.T) {
	ctx := t.Context()
	settings := receivertest.NewNopSettings(metadata.Type)

	td := &testData{
		name: "test_offset_target",
		pages: []mockPrometheusResponse{
			{
				code: 200,
				data: `
# HELP test_gauge A gauge for testing
# TYPE test_gauge gauge
test_gauge 42
`,
			},
		},
	}

	mp, promCfg, err := setupMockPrometheus(td)
	require.NoError(t, err)
	defer mp.Close()

	cfg := &Config{
		PrometheusConfig:    promCfg,
		InitialScrapeOffset: 1500 * time.Millisecond,
		skipOffsetting:      true,
	}

	sink := new(consumertest.MetricsSink)
	r, err := newPrometheusReceiver(settings, cfg, sink)
	require.NoError(t, err)

	startTime := time.Now()

	err = r.start(ctx, componenttest.NewNopHost(), prometheusComponentTestOptions{
		discovery: prometheusDiscoveryTestOptions{updatert: 50 * time.Millisecond},
		scrape:    prometheusScrapeTestOptions{discoveryReloadInterval: 50 * time.Millisecond},
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(sink.AllMetrics()) >= 1
	}, 5*time.Second, 10*time.Millisecond)

	elapsed := time.Since(startTime)
	assert.GreaterOrEqual(t, elapsed, 1500*time.Millisecond, "First scrape should be delayed by at least the configured InitialScrapeOffset (1500ms)")

	require.NoError(t, r.Shutdown(ctx))
}
