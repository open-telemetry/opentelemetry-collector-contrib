// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package cpuscraper

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
)

func TestScrape_CpuFrequency(t *testing.T) {
	type testCase struct {
		name             string
		enabledFrequency bool
	}

	testCases := []testCase{
		{
			name:             "System CPU Frequency enabled",
			enabledFrequency: true,
		},
		{
			name:             "System CPU Frequency disabled",
			enabledFrequency: false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := metadata.DefaultMetricsBuilderConfig()
			cfg.Metrics.SystemCPUTime.Enabled = false
			cfg.Metrics.SystemCPUFrequency.Enabled = test.enabledFrequency

			scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type),
				&Config{MetricsBuilderConfig: cfg})

			err := scraper.start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize CPU scraper: %v", err)

			md, err := scraper.scrape(t.Context())
			require.NoError(t, err, "Failed to scrape metrics: %v", err)

			expectedMetricCount := 0
			if test.enabledFrequency {
				expectedMetricCount++
			}

			require.Equal(t, expectedMetricCount, md.MetricCount(),
				"Expected %d metrics but got %d", expectedMetricCount, md.MetricCount())

			if expectedMetricCount == 0 {
				return
			}

			metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			reportedMetrics := make(map[string]int)

			for _, metric := range metrics.All() {
				reportedMetrics[metric.Name()]++

				switch metric.Name() {
				case "system.cpu.frequency":
					require.True(t, test.enabledFrequency,
						"CPU frequency metric found but test expects it disabled")
					assertCPUFrequencyMetricValid(t, metric)
				default:
					require.Fail(t, "unexpected-metric", "Unexpected metric %q found", metric.Name())
				}
			}

			for metricName, count := range reportedMetrics {
				require.Equal(t, 1, count, "Metric %q reported %d times, expected 1", metricName, count)
			}

			if test.enabledFrequency {
				_, found := reportedMetrics["system.cpu.frequency"]
				require.True(t, found, "CPU frequency metric is enabled but not found")
			}
		})
	}
}

func assertCPUFrequencyMetricValid(t *testing.T, metric pmetric.Metric) {
	expected := pmetric.NewMetric()
	expected.SetName("system.cpu.frequency")
	expected.SetDescription("Current frequency of the CPU core in Hz.")
	expected.SetUnit("Hz")
	expected.SetEmptyGauge()
	internal.AssertDescriptorEqual(t, expected, metric)

	numCPUs := runtime.NumCPU()
	require.GreaterOrEqual(t, metric.Gauge().DataPoints().Len(), numCPUs,
		"Should have at least one frequency data point per CPU")

	if metric.Gauge().DataPoints().Len() > 0 {
		dp := metric.Gauge().DataPoints().At(0)

		cpuAttr, exists := dp.Attributes().Get("cpu")
		require.True(t, exists, "Data point should have 'cpu' attribute")
		require.Contains(t, cpuAttr.Str(), "cpu", "CPU attribute should contain 'cpu' prefix")

		require.GreaterOrEqual(t, dp.DoubleValue(), 0.0, "CPU frequency should be non-negative")
	}
}
