// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package cpuscraper

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
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

			cfg := metadata.NewDefaultMetricsBuilderConfig()
			cfg.Metrics.SystemCPUTime.Enabled = false
			cfg.Metrics.SystemCPULogicalCount.Enabled = false
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

	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.SystemCPULogicalCount.Enabled = false
	cfg.Metrics.SystemCPUTime.EnabledAttributes = []metadata.SystemCPUTimeMetricAttributeKey{
		metadata.SystemCPUTimeMetricAttributeKeyCPU,
		metadata.SystemCPUTimeMetricAttributeKeyState,
	}
	cfg.Metrics.SystemCPUUtilization.Enabled = true
	cfg.Metrics.SystemCPUUtilization.EnabledAttributes = []metadata.SystemCPUUtilizationMetricAttributeKey{
		metadata.SystemCPUUtilizationMetricAttributeKeyCPU,
		metadata.SystemCPUUtilizationMetricAttributeKeyState,
	}

	t.Setenv("HOST_PROC", procDir)
	scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type),
		&Config{MetricsBuilderConfig: cfg})
	scraper.emitCPUMetrics = newCputicksEmitter(cpu.NewExLinux().TimesWithContext)
	scraper.bootTime = func(context.Context) (uint64, error) { return 100, nil }

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

	scrape1 := "cpu  100 25 50 200 25 25 25 50 0 0\ncpu0 100 25 50 200 25 25 25 50 0 0\n"
	scrape2 := "cpu  300 75 150 600 75 75 75 150 0 0\ncpu0 300 75 150 600 75 75 75 150 0 0\n"

	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.SystemCPULogicalCount.Enabled = false
	cfg.Metrics.SystemCPUTime.EnabledAttributes = []metadata.SystemCPUTimeMetricAttributeKey{
		metadata.SystemCPUTimeMetricAttributeKeyCPU,
		metadata.SystemCPUTimeMetricAttributeKeyState,
	}
	cfg.Metrics.SystemCPUUtilization.Enabled = true
	cfg.Metrics.SystemCPUUtilization.EnabledAttributes = []metadata.SystemCPUUtilizationMetricAttributeKey{
		metadata.SystemCPUUtilizationMetricAttributeKeyCPU,
		metadata.SystemCPUUtilizationMetricAttributeKeyState,
	}

	t.Setenv("HOST_PROC", procDir)
	scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type),
		&Config{MetricsBuilderConfig: cfg})
	scraper.emitCPUMetrics = newCputicksEmitter(cpu.NewExLinux().TimesWithContext)
	scraper.bootTime = func(context.Context) (uint64, error) { return 100, nil }

	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(procDir, "stat"), []byte(scrape1), 0o600))
	_, err = scraper.scrape(t.Context())
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(procDir, "stat"), []byte(scrape2), 0o600))
	md, err := scraper.scrape(t.Context())
	require.NoError(t, err)

	require.Equal(t, 2, md.MetricCount())

	expectedFile := filepath.Join("testdata", "cputicks_utilization_expected.yaml")
	expected, err := golden.ReadMetrics(expectedFile)
	require.NoError(t, err)
	require.NoError(t, pmetrictest.CompareMetrics(expected, md,
		pmetrictest.IgnoreTimestamp(),
		pmetrictest.IgnoreStartTimestamp(),
		pmetrictest.IgnoreMetricsOrder(),
		pmetrictest.IgnoreMetricDataPointsOrder(),
	))
}

func TestCputicksEmitter_NewCPUAppears(t *testing.T) {
	root := t.TempDir()
	procDir := filepath.Join(root, "proc")
	require.NoError(t, os.MkdirAll(procDir, 0o755))

	scrape1 := "cpu  100 25 50 200 25 25 25 50 0 0\ncpu0 100 25 50 200 25 25 25 50 0 0\n"
	scrape2 := "cpu  300 75 150 600 75 75 75 150 0 0\ncpu0 200 50 100 400 50 50 50 100 0 0\ncpu1 100 25 50 200 25 25 25 50 0 0\n"

	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.SystemCPUUtilization.Enabled = true

	t.Setenv("HOST_PROC", procDir)
	scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type),
		&Config{MetricsBuilderConfig: cfg})
	scraper.emitCPUMetrics = newCputicksEmitter(cpu.NewExLinux().TimesWithContext)
	scraper.bootTime = func(context.Context) (uint64, error) { return 100, nil }

	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(procDir, "stat"), []byte(scrape1), 0o600))
	_, err = scraper.scrape(t.Context())
	require.NoError(t, err)

	// New CPU appears. Should be silently skipped for utilization.
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "stat"), []byte(scrape2), 0o600))
	_, err = scraper.scrape(t.Context())
	require.NoError(t, err)
}

func TestCputicksEmitter_CPUDisappears(t *testing.T) {
	root := t.TempDir()
	procDir := filepath.Join(root, "proc")
	require.NoError(t, os.MkdirAll(procDir, 0o755))

	scrape1 := "cpu  200 50 100 400 50 50 50 100 0 0\ncpu0 100 25 50 200 25 25 25 50 0 0\ncpu1 100 25 50 200 25 25 25 50 0 0\n"
	scrape2 := "cpu  300 75 150 600 75 75 75 150 0 0\ncpu0 300 75 150 600 75 75 75 150 0 0\n"

	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.SystemCPUUtilization.Enabled = true

	t.Setenv("HOST_PROC", procDir)
	scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type),
		&Config{MetricsBuilderConfig: cfg})
	scraper.emitCPUMetrics = newCputicksEmitter(cpu.NewExLinux().TimesWithContext)
	scraper.bootTime = func(context.Context) (uint64, error) { return 100, nil }

	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(procDir, "stat"), []byte(scrape1), 0o600))
	_, err = scraper.scrape(t.Context())
	require.NoError(t, err)

	// CPU disappears. Should return error.
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "stat"), []byte(scrape2), 0o600))
	_, err = scraper.scrape(t.Context())
	var partialErr scrapererror.PartialScrapeError
	require.ErrorAs(t, err, &partialErr)
	assert.ErrorContains(t, partialErr, ucal.ErrTimeStatNotFound.Error())
}

func TestCputicksEmitter_ReadError(t *testing.T) {
	readErr := errors.New("disk I/O error")

	cfg := metadata.NewDefaultMetricsBuilderConfig()
	scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type),
		&Config{MetricsBuilderConfig: cfg})
	scraper.emitCPUMetrics = newCputicksEmitter(func(_ context.Context, _ bool) ([]cpu.ExTimesStat, error) {
		return nil, readErr
	})

	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	_, err = scraper.scrape(t.Context())
	var partialErr scrapererror.PartialScrapeError
	require.ErrorAs(t, err, &partialErr)
	assert.ErrorContains(t, partialErr, "disk I/O error")
}

func TestCputicksEmitter_ZeroDelta(t *testing.T) {
	sameTicks := []cpu.ExTimesStat{
		{CPU: "cpu0", User: 100, Nice: 25, System: 50, Idle: 200, Iowait: 25, Irq: 25, Softirq: 25, Steal: 50},
	}

	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.SystemCPULogicalCount.Enabled = false
	cfg.Metrics.SystemCPUUtilization.Enabled = true

	scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type),
		&Config{MetricsBuilderConfig: cfg})
	scraper.emitCPUMetrics = newCputicksEmitter(func(_ context.Context, _ bool) ([]cpu.ExTimesStat, error) {
		return sameTicks, nil
	})

	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err)

	_, err = scraper.scrape(t.Context())
	require.NoError(t, err)

	md, err := scraper.scrape(t.Context())
	require.NoError(t, err)
	require.Equal(t, 2, md.MetricCount())

	metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
	for _, m := range metrics.All() {
		if m.Name() == "system.cpu.utilization" {
			dp := m.Gauge().DataPoints()
			require.Equal(t, 8, dp.Len(), "expected 8 states for 1 CPU")
			for i := range dp.Len() {
				assert.Equal(t, 0.0, dp.At(i).DoubleValue(), "all utilization should be 0 when delta is 0")
			}
			return
		}
	}
	t.Fatal("system.cpu.utilization metric not found")
}

func TestCputicksEmitter_DecreasingCounter(t *testing.T) {
	tests := []struct {
		name     string
		prev     cpu.ExTimesStat
		curr     cpu.ExTimesStat
		expected map[string]float64
	}{
		{
			name: "iowait decreases",
			prev: cpu.ExTimesStat{CPU: "cpu0", User: 100, System: 100, Idle: 700, Iowait: 100},
			curr: cpu.ExTimesStat{CPU: "cpu0", User: 150, System: 150, Idle: 800, Iowait: 90},
			expected: map[string]float64{
				"user": 0.25, "system": 0.25, "idle": 0.5, "wait": 0,
				"interrupt": 0, "nice": 0, "softirq": 0, "steal": 0,
			},
		},
		{
			name: "all counters decrease",
			prev: cpu.ExTimesStat{CPU: "cpu0", User: 500, Nice: 50, System: 500, Idle: 900, Iowait: 100, Irq: 10, Softirq: 10, Steal: 5},
			curr: cpu.ExTimesStat{CPU: "cpu0", User: 1, Nice: 1, System: 1, Idle: 1, Iowait: 1, Irq: 1, Softirq: 1, Steal: 1},
			expected: map[string]float64{
				"user": 0, "system": 0, "idle": 0, "wait": 0,
				"interrupt": 0, "nice": 0, "softirq": 0, "steal": 0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := metadata.NewDefaultMetricsBuilderConfig()
			cfg.Metrics.SystemCPULogicalCount.Enabled = false
			cfg.Metrics.SystemCPUUtilization.Enabled = true

			reads := [][]cpu.ExTimesStat{{tt.prev}, {tt.curr}}
			scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type),
				&Config{MetricsBuilderConfig: cfg})
			scraper.emitCPUMetrics = newCputicksEmitter(func(_ context.Context, _ bool) ([]cpu.ExTimesStat, error) {
				ticks := reads[0]
				reads = reads[1:]
				return ticks, nil
			})
			require.NoError(t, scraper.start(t.Context(), componenttest.NewNopHost()))

			_, err := scraper.scrape(t.Context())
			require.NoError(t, err)
			md, err := scraper.scrape(t.Context())
			require.NoError(t, err)

			actual := map[string]float64{}
			for _, m := range md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().All() {
				if m.Name() != "system.cpu.utilization" {
					continue
				}
				for _, dp := range m.Gauge().DataPoints().All() {
					state, ok := dp.Attributes().Get("state")
					require.True(t, ok)
					actual[state.Str()] = dp.DoubleValue()
				}
			}
			assert.Equal(t, tt.expected, actual)
		})
	}
}
