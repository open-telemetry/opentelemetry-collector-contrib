// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package cpuscraper

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/ucal"
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
			metric := metrics.At(0)
			assertCPUFrequencyMetricValid(t, metric)
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

func TestCputicksEmitter(t *testing.T) {
	root := t.TempDir()
	procDir := filepath.Join(root, "proc")
	require.NoError(t, os.MkdirAll(procDir, 0o755))

	scrape1 := "cpu  100 200 300 400 50 60 70 80 0 0\ncpu0 50 100 150 200 25 30 35 40 0 0\ncpu1 50 100 150 200 25 30 35 40 0 0\n"
	scrape2 := "cpu  200 400 600 800 100 120 140 160 0 0\ncpu0 110 120 170 220 35 40 45 50 0 0\ncpu1 90 280 430 580 65 80 95 110 0 0\n"

	cfg := metadata.DefaultMetricsBuilderConfig()
	cfg.Metrics.SystemCPUUtilization.Enabled = true

	scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type),
		&Config{MetricsBuilderConfig: cfg, rootPath: root})
	scraper.emitCPUMetrics = newCputicksEmitter(root)

	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(procDir, "stat"), []byte(scrape1), 0o600))
	md, err := scraper.scrape(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, md.MetricCount(), "first scrape should only have cpu.time (no utilization yet)")

	require.NoError(t, os.WriteFile(filepath.Join(procDir, "stat"), []byte(scrape2), 0o600))
	md, err = scraper.scrape(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 2, md.MetricCount(), "second scrape should have cpu.time and cpu.utilization")

	metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	var utilMetric pmetric.Metric
	for _, m := range metrics.All() {
		if m.Name() == "system.cpu.utilization" {
			utilMetric = m
			break
		}
	}
	require.NotEqual(t, pmetric.Metric{}, utilMetric, "utilization metric not found")

	dp := utilMetric.Gauge().DataPoints()
	require.Equal(t, 16, dp.Len(), "expected 8 states * 2 CPUs")

	for i := range dp.Len() {
		val := dp.At(i).DoubleValue()
		assert.GreaterOrEqual(t, val, 0.0, "utilization should be >= 0")
		assert.LessOrEqual(t, val, 1.0, "utilization should be <= 1")
	}
}

func TestCputicksEmitter_Utilization(t *testing.T) {
	root := t.TempDir()
	procDir := filepath.Join(root, "proc")
	require.NoError(t, os.MkdirAll(procDir, 0o755))

	scrape1 := "cpu  100 0 100 800 0 0 0 0 0 0\ncpu0 100 0 100 800 0 0 0 0 0 0\n"
	scrape2 := "cpu  200 0 200 1600 0 0 0 0 0 0\ncpu0 200 0 200 1600 0 0 0 0 0 0\n"

	cfg := metadata.DefaultMetricsBuilderConfig()
	cfg.Metrics.SystemCPUUtilization.Enabled = true

	scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type),
		&Config{MetricsBuilderConfig: cfg, rootPath: root})
	scraper.emitCPUMetrics = newCputicksEmitter(root)

	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(procDir, "stat"), []byte(scrape1), 0o600))
	_, err = scraper.scrape(t.Context())
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(procDir, "stat"), []byte(scrape2), 0o600))
	md, err := scraper.scrape(t.Context())
	require.NoError(t, err)

	require.Equal(t, 2, md.MetricCount())
	metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()

	findMetric := func(name string) pmetric.Metric {
		for _, m := range metrics.All() {
			if m.Name() == name {
				return m
			}
		}
		t.Fatalf("metric not found: %s", name)
		return pmetric.Metric{}
	}

	findDP := func(metric pmetric.Metric, cpuName, state string) float64 {
		var dp pmetric.NumberDataPointSlice
		switch metric.Type() {
		case pmetric.MetricTypeSum:
			dp = metric.Sum().DataPoints()
		case pmetric.MetricTypeGauge:
			dp = metric.Gauge().DataPoints()
		}
		for i := range dp.Len() {
			p := dp.At(i)
			cpuAttr, _ := p.Attributes().Get("cpu")
			stateAttr, _ := p.Attributes().Get("state")
			if cpuAttr.Str() == cpuName && stateAttr.Str() == state {
				return p.DoubleValue()
			}
		}
		t.Fatalf("datapoint not found: metric=%s cpu=%s state=%s", metric.Name(), cpuName, state)
		return 0
	}

	timeMetric := findMetric("system.cpu.time")
	assert.InDelta(t, 2.0, findDP(timeMetric, "cpu0", "user"), 0.001)
	assert.InDelta(t, 2.0, findDP(timeMetric, "cpu0", "system"), 0.001)
	assert.InDelta(t, 16.0, findDP(timeMetric, "cpu0", "idle"), 0.001)

	utilMetric := findMetric("system.cpu.utilization")
	assert.InDelta(t, 0.1, findDP(utilMetric, "cpu0", "user"), 0.001)
	assert.InDelta(t, 0.1, findDP(utilMetric, "cpu0", "system"), 0.001)
	assert.InDelta(t, 0.8, findDP(utilMetric, "cpu0", "idle"), 0.001)
	assert.InDelta(t, 0.0, findDP(utilMetric, "cpu0", "nice"), 0.001)
}

func TestCputicksEmitter_NewCPUAppears(t *testing.T) {
	root := t.TempDir()
	procDir := filepath.Join(root, "proc")
	require.NoError(t, os.MkdirAll(procDir, 0o755))

	scrape1 := "cpu  100 0 100 800 0 0 0 0 0 0\ncpu0 100 0 100 800 0 0 0 0 0 0\n"
	scrape2 := "cpu  200 0 200 1600 0 0 0 0 0 0\ncpu0 150 0 150 1200 0 0 0 0 0 0\ncpu1 50 0 50 400 0 0 0 0 0 0\n"

	cfg := metadata.DefaultMetricsBuilderConfig()
	cfg.Metrics.SystemCPUUtilization.Enabled = true

	scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type),
		&Config{MetricsBuilderConfig: cfg, rootPath: root})
	scraper.emitCPUMetrics = newCputicksEmitter(root)

	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(procDir, "stat"), []byte(scrape1), 0o600))
	_, err = scraper.scrape(t.Context())
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(procDir, "stat"), []byte(scrape2), 0o600))
	_, err = scraper.scrape(t.Context())
	require.ErrorIs(t, err, ucal.ErrTimeStatNotFound)
}

