// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package loadscraper

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/scraper/scrapertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper/internal/metadata"
)

func TestStopSamplingWithoutStart(t *testing.T) {
	// When the collector fails to start it is possible that stopSampling is called
	// before startSampling. This test ensures that stopSampling does not panic in
	// this scenario.
	require.NoError(t, stopSampling(context.Background()))
}

func Benchmark_SampleLoad(b *testing.B) {
	s, _ := newSampler(zap.NewNop())

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		s.sampleLoad()
	}
}

func TestSetSkipScrapeOnFailureToStart(t *testing.T) {
	originalPerfCounterFactory := perfCounterFactory
	defer func() {
		perfCounterFactory = originalPerfCounterFactory
	}()

	perfCounterFactory = func(_ string, _ string, _ string) (winperfcounters.PerfCounterWatcher, error) {
		return nil, errors.New("error creating perf counter watcher")
	}

	scraper := newLoadScraper(
		context.Background(),
		scrapertest.NewNopSettings(metadata.Type),
		&Config{
			MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
		},
	)
	require.NotNil(t, scraper)

	require.NoError(t, scraper.start(context.Background(), componenttest.NewNopHost()))
	defer func() {
		assert.NoError(t, scraper.shutdown(context.Background()))
	}()

	assert.True(t, scraper.skipScrape)
	metrics, err := scraper.scrape(context.Background())
	assert.NoError(t, err)
	assert.Zero(t, metrics.MetricCount())
}

func TestLoadScrapeWithRealData(t *testing.T) {
	config := Config{
		MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}
	scraper := newLoadScraper(context.Background(), scrapertest.NewNopSettings(metadata.Type), &config)

	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err, "Failed to start the load scraper")
	defer func() {
		assert.NoError(t, scraper.shutdown(context.Background()), "Failed to shutdown the load scraper")
	}()

	metrics, err := scraper.scrape(context.Background())
	require.NoError(t, err, "Failed to scrape metrics")
	require.NotNil(t, metrics, "Metrics cannot be nil")

	// Expected metric names for load scraper
	expectedMetrics := map[string]bool{
		"system.cpu.load_average.1m":  false,
		"system.cpu.load_average.5m":  false,
		"system.cpu.load_average.15m": false,
	}

	internal.AssertExpectedMetrics(t, expectedMetrics, metrics)
}
