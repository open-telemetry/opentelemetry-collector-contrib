// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package cpuscraper

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
)

func TestScrape_CpuFrequency(t *testing.T) {
	type testCase struct {
		name                string
		metricsConfig       metadata.MetricsBuilderConfig
		expectedMetricCount int
	}

	testCases := []testCase{
		{
			name: "System CPU Frequency enabled",
			metricsConfig: metadata.MetricsBuilderConfig{
				Metrics: metadata.MetricsConfig{
					SystemCPUTime: metadata.MetricConfig{
						Enabled: false,
					},
					SystemCPUFrequency: metadata.MetricConfig{
						Enabled: true,
					},
				},
			},
			expectedMetricCount: 1,
		},
		{
			name: "System CPU Frequency disabled",
			metricsConfig: metadata.MetricsBuilderConfig{
				Metrics: metadata.MetricsConfig{
					SystemCPUTime: metadata.MetricConfig{
						Enabled: false,
					},
					SystemCPUFrequency: metadata.MetricConfig{
						Enabled: false,
					},
				},
			},
			expectedMetricCount: 0,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), &Config{MetricsBuilderConfig: test.metricsConfig})

			err := scraper.start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize CPU scraper: %v", err)

			md, err := scraper.scrape(t.Context())
			require.NoError(t, err, "Failed to scrape metrics: %v", err)

			assert.Equal(t, test.expectedMetricCount, md.MetricCount())

			if md.MetricCount() > 0 {
				metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
				metric := metrics.At(0)

				assertCPUFrequencyMetricValid(t, metric)
			}
		})
	}
}

func assertCPUFrequencyMetricValid(t *testing.T, metric pmetric.Metric) {
	assert.Equal(t, "system.cpu.frequency", metric.Name())
	assert.Equal(t, "Current frequency of the CPU core in Hz.", metric.Description())
	assert.Equal(t, "Hz", metric.Unit())
	assert.Equal(t, pmetric.MetricTypeGauge, metric.Type())

	numCPUs := runtime.NumCPU()
	assert.GreaterOrEqual(t, metric.Gauge().DataPoints().Len(), numCPUs,
		"Should have at least one frequency data point per CPU")

	if metric.Gauge().DataPoints().Len() > 0 {
		dp := metric.Gauge().DataPoints().At(0)

		cpuAttr, exists := dp.Attributes().Get("cpu")
		assert.True(t, exists, "Data point should have 'cpu' attribute")
		assert.Contains(t, cpuAttr.Str(), "cpu", "CPU attribute should contain 'cpu' prefix")

		assert.Greater(t, dp.DoubleValue(), 0.0, "CPU frequency should be greater than 0")
	}
}
