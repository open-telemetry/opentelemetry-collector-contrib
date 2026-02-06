// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cpuscraper

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	type testCase struct {
		name                string
		bootTimeFunc        func(context.Context) (uint64, error)
		timesFunc           func(context.Context, bool) ([]cpu.TimesStat, error)
		metricsConfig       metadata.MetricsBuilderConfig
		expectedMetricCount int
		expectedStartTime   pcommon.Timestamp
		initializationErr   string
		expectedErr         string
	}

	disabledMetric := metadata.DefaultMetricsBuilderConfig()
	disabledMetric.Metrics.SystemCPUTime.Enabled = false

	testCases := []testCase{
		{
			name:                "Standard",
			metricsConfig:       metadata.DefaultMetricsBuilderConfig(),
			expectedMetricCount: 1,
		},
		{
			name:                "Validate Start Time",
			bootTimeFunc:        func(context.Context) (uint64, error) { return 100, nil },
			metricsConfig:       metadata.DefaultMetricsBuilderConfig(),
			expectedMetricCount: 1,
			expectedStartTime:   100 * 1e9,
		},
		{
			name:                "Boot Time Error",
			bootTimeFunc:        func(context.Context) (uint64, error) { return 0, errors.New("err1") },
			metricsConfig:       metadata.DefaultMetricsBuilderConfig(),
			expectedMetricCount: 1,
			initializationErr:   "err1",
		},
		{
			name:                "Times Error",
			timesFunc:           func(context.Context, bool) ([]cpu.TimesStat, error) { return nil, errors.New("err2") },
			metricsConfig:       metadata.DefaultMetricsBuilderConfig(),
			expectedMetricCount: 1,
			expectedErr:         "err2",
		},
		{
			name:                "SystemCPUTime metric is disabled ",
			metricsConfig:       disabledMetric,
			expectedMetricCount: 0,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), &Config{MetricsBuilderConfig: test.metricsConfig})
			if test.bootTimeFunc != nil {
				scraper.bootTime = test.bootTimeFunc
			}
			if test.timesFunc != nil {
				scraper.times = test.timesFunc
			}

			err := scraper.start(t.Context(), componenttest.NewNopHost())
			if test.initializationErr != "" {
				assert.EqualError(t, err, test.initializationErr)
				return
			}
			require.NoError(t, err, "Failed to initialize cpu scraper: %v", err)

			md, err := scraper.scrape(t.Context())
			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)

				isPartial := scrapererror.IsPartialScrapeError(err)
				assert.True(t, isPartial)
				if isPartial {
					var scraperErr scrapererror.PartialScrapeError
					require.ErrorAs(t, err, &scraperErr)
					assert.Equal(t, 2, scraperErr.Failed)
				}

				return
			}
			require.NoError(t, err, "Failed to scrape metrics: %v", err)

			assert.Equal(t, test.expectedMetricCount, md.MetricCount())

			if test.expectedMetricCount > 0 {
				metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
				assertCPUMetricValid(t, metrics.At(0), test.expectedStartTime)

				if runtime.GOOS == "linux" {
					assertCPUMetricHasLinuxSpecificStateLabels(t, metrics.At(0))
				}

				internal.AssertSameTimeStampForAllMetrics(t, metrics)
			}
		})
	}
}

func TestScrape_CpuCount(t *testing.T) {
	type testCase struct {
		name                 string
		enabledPhysicalCount bool
		enabledLogicalCount  bool
	}

	testCases := []testCase{
		{
			name:                 "Both Physical and Logical CPU count enabled",
			enabledPhysicalCount: true,
			enabledLogicalCount:  true,
		},
		{
			name:                 "Physical CPU count enabled",
			enabledPhysicalCount: true,
			enabledLogicalCount:  false,
		},
		{
			name:                 "Logical CPU count enabled",
			enabledPhysicalCount: false,
			enabledLogicalCount:  true,
		},
		{
			name:                 "Both Physical and Logical CPU count disabled",
			enabledPhysicalCount: false,
			enabledLogicalCount:  false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := metadata.DefaultMetricsBuilderConfig()
			cfg.Metrics.SystemCPUTime.Enabled = false
			cfg.Metrics.SystemCPUPhysicalCount.Enabled = test.enabledPhysicalCount
			cfg.Metrics.SystemCPULogicalCount.Enabled = test.enabledLogicalCount

			scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), &Config{MetricsBuilderConfig: cfg})

			err := scraper.start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize CPU scraper: %v", err)

			md, err := scraper.scrape(t.Context())
			require.NoError(t, err, "Failed to scrape metrics: %v", err)

			expectedMetricCount := 0
			if test.enabledPhysicalCount {
				expectedMetricCount++
			}
			if test.enabledLogicalCount {
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
				case "system.cpu.physical.count":
					require.True(t, test.enabledPhysicalCount, "Physical count metric found but test expects it disabled")
					assertCPUPhysicalCountMetricValid(t, metric)
				case "system.cpu.logical.count":
					require.True(t, test.enabledLogicalCount, "Logical count metric found but test expects it disabled")
					assertCPULogicalCountMetricValid(t, metric)
				default:
					require.Fail(t, "unexpected-metric", "Unexpected metric %q found", metric.Name())
				}
			}

			for metricName, count := range reportedMetrics {
				require.Equal(t, 1, count, "Metric %q reported %d times, expected 1", metricName, count)
			}

			if test.enabledPhysicalCount {
				_, found := reportedMetrics["system.cpu.physical.count"]
				require.True(t, found, "Physical count metric is enabled but not found")
			}
			if test.enabledLogicalCount {
				_, found := reportedMetrics["system.cpu.logical.count"]
				require.True(t, found, "Logical count metric is enabled but not found")
			}
		})
	}
}

// TestScrape_CpuUtilization to test utilization we need to execute scrape at least twice to have
// data to calculate the difference, so assertions will be done after the second scraping
func TestScrape_CpuUtilization(t *testing.T) {
	type testCase struct {
		name                string
		metricsConfig       metadata.MetricsBuilderConfig
		expectedMetricCount int
		times               bool
		utilization         bool
		utilizationIndex    int
	}

	testCases := []testCase{
		{
			name:                "Standard",
			metricsConfig:       metadata.DefaultMetricsBuilderConfig(),
			expectedMetricCount: 1,
			times:               true,
			utilization:         false,
		},
		{
			name:                "SystemCPUTime metric is disabled",
			times:               false,
			utilization:         true,
			expectedMetricCount: 1,
		},
		{
			name:                "all metrics are enabled",
			times:               true,
			utilization:         true,
			expectedMetricCount: 2,
			utilizationIndex:    1,
		},
		{
			name:                "all metrics are disabled",
			times:               false,
			utilization:         false,
			expectedMetricCount: 0,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			settings := test.metricsConfig
			if test.metricsConfig.Metrics == (metadata.MetricsConfig{}) {
				settings = metadata.DefaultMetricsBuilderConfig()
				settings.Metrics.SystemCPUTime.Enabled = test.times
				settings.Metrics.SystemCPUUtilization.Enabled = test.utilization
			}

			scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), &Config{MetricsBuilderConfig: settings})
			err := scraper.start(t.Context(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize cpu scraper: %v", err)

			_, err = scraper.scrape(t.Context())
			require.NoError(t, err, "Failed to scrape metrics: %v", err)
			// 2nd scrape will trigger utilization metrics calculation
			md, err := scraper.scrape(t.Context())
			require.NoError(t, err, "Failed to scrape metrics: %v", err)

			assert.Equal(t, test.expectedMetricCount, md.MetricCount())
			if md.ResourceMetrics().Len() == 0 {
				return
			}

			metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()
			internal.AssertSameTimeStampForAllMetrics(t, metrics)
			if test.times {
				timesMetrics := metrics.At(0)
				assertCPUMetricValid(t, timesMetrics, 0)
				if runtime.GOOS == "linux" {
					assertCPUMetricHasLinuxSpecificStateLabels(t, timesMetrics)
				}
			}
			if test.utilization {
				utilizationMetrics := metrics.At(test.utilizationIndex)
				assertCPUUtilizationMetricValid(t, utilizationMetrics, 0)
				if runtime.GOOS == "linux" {
					assertCPUUtilizationMetricHasLinuxSpecificStateLabels(t, utilizationMetrics)
				}
			}
		})
	}
}

// Error in calculation should be returned as PartialScrapeError
func TestScrape_CpuUtilizationError(t *testing.T) {
	scraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), &Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()})
	// mock times function to force an error in next scrape
	scraper.times = func(context.Context, bool) ([]cpu.TimesStat, error) {
		return []cpu.TimesStat{{CPU: "1", System: 1, User: 2}}, nil
	}
	err := scraper.start(t.Context(), componenttest.NewNopHost())
	require.NoError(t, err, "Failed to initialize cpu scraper: %v", err)

	_, err = scraper.scrape(t.Context())
	// Force error not finding CPU info
	scraper.times = func(context.Context, bool) ([]cpu.TimesStat, error) {
		return []cpu.TimesStat{}, nil
	}
	require.NoError(t, err, "Failed to scrape metrics: %v", err)
	// 2nd scrape will trigger utilization metrics calculation
	md, err := scraper.scrape(t.Context())
	var partialScrapeErr scrapererror.PartialScrapeError
	assert.ErrorAs(t, err, &partialScrapeErr)
	assert.Equal(t, 0, md.MetricCount())
}

func TestScrape_CpuUtilizationStandard(t *testing.T) {
	overriddenMetricsSettings := metadata.DefaultMetricsBuilderConfig()
	overriddenMetricsSettings.Metrics.SystemCPUUtilization.Enabled = true
	overriddenMetricsSettings.Metrics.SystemCPUTime.Enabled = false

	// datapoint data
	type dpData struct {
		val   float64
		attrs map[string]string
	}

	scrapesData := []struct {
		times       []cpu.TimesStat
		scrapeTime  string
		expectedDps []dpData
	}{
		{
			times:       []cpu.TimesStat{{CPU: "cpu0", User: 1.5, System: 2.7, Idle: 0.8}, {CPU: "cpu1", User: 2, System: 3, Idle: 1}},
			scrapeTime:  "2006-01-02T15:04:05Z",
			expectedDps: []dpData{},
		},
		{
			times:      []cpu.TimesStat{{CPU: "cpu0", User: 2.8, System: 3.9, Idle: 3.3}, {CPU: "cpu1", User: 3.2, System: 5.2, Idle: 2.6}},
			scrapeTime: "2006-01-02T15:04:10Z",
			expectedDps: []dpData{
				{val: 0.26, attrs: map[string]string{"cpu": "cpu0", "state": "user"}},
				{val: 0.24, attrs: map[string]string{"cpu": "cpu0", "state": "system"}},
				{val: 0.5, attrs: map[string]string{"cpu": "cpu0", "state": "idle"}},
				{val: 0.24, attrs: map[string]string{"cpu": "cpu1", "state": "user"}},
				{val: 0.44, attrs: map[string]string{"cpu": "cpu1", "state": "system"}},
				{val: 0.32, attrs: map[string]string{"cpu": "cpu1", "state": "idle"}},
			},
		},
		{
			times:      []cpu.TimesStat{{CPU: "cpu0", User: 3.4, System: 5.3, Idle: 6.3}, {CPU: "cpu1", User: 3.7, System: 7.1, Idle: 5.2}},
			scrapeTime: "2006-01-02T15:04:15Z",
			expectedDps: []dpData{
				{val: 0.12, attrs: map[string]string{"cpu": "cpu0", "state": "user"}},
				{val: 0.28, attrs: map[string]string{"cpu": "cpu0", "state": "system"}},
				{val: 0.6, attrs: map[string]string{"cpu": "cpu0", "state": "idle"}},
				{val: 0.1, attrs: map[string]string{"cpu": "cpu1", "state": "user"}},
				{val: 0.38, attrs: map[string]string{"cpu": "cpu1", "state": "system"}},
				{val: 0.52, attrs: map[string]string{"cpu": "cpu1", "state": "idle"}},
			},
		},
	}

	cpuScraper := newCPUScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), &Config{MetricsBuilderConfig: overriddenMetricsSettings})
	for _, scrapeData := range scrapesData {
		// mock TimeStats and Now
		cpuScraper.times = func(context.Context, bool) ([]cpu.TimesStat, error) {
			return scrapeData.times, nil
		}
		cpuScraper.now = func() time.Time {
			now, _ := time.Parse(time.RFC3339, scrapeData.scrapeTime)
			return now
		}

		err := cpuScraper.start(t.Context(), componenttest.NewNopHost())
		require.NoError(t, err, "Failed to initialize cpu scraper: %v", err)

		md, err := cpuScraper.scrape(t.Context())
		require.NoError(t, err)
		// no metrics in the first scrape
		if len(scrapeData.expectedDps) == 0 {
			assert.Equal(t, 0, md.ResourceMetrics().Len())
			continue
		}

		assert.Equal(t, 1, md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
		metric := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
		assertCPUUtilizationMetricValid(t, metric, 0)
		dp := metric.Gauge().DataPoints()

		expectedDataPoints := 8
		if runtime.GOOS == "linux" {
			expectedDataPoints = 16
			assertCPUUtilizationMetricHasLinuxSpecificStateLabels(t, metric)
		}
		assert.Equal(t, expectedDataPoints, dp.Len())

		// remove empty values to make the test more simple
		dp.RemoveIf(func(n pmetric.NumberDataPoint) bool {
			return n.DoubleValue() == 0.0
		})

		for idx, expectedDp := range scrapeData.expectedDps {
			assertDatapointValueAndStringAttributes(t, dp.At(idx), expectedDp.val, expectedDp.attrs)
		}
	}
}

func assertDatapointValueAndStringAttributes(t *testing.T, dp pmetric.NumberDataPoint, value float64, attrs map[string]string) {
	assert.InDelta(t, value, dp.DoubleValue(), 0.0001)
	for k, v := range attrs {
		cpuAttribute, exists := dp.Attributes().Get(k)
		assert.True(t, exists)
		assert.Equal(t, v, cpuAttribute.Str())
	}
}

func assertCPUMetricValid(t *testing.T, metric pmetric.Metric, startTime pcommon.Timestamp) {
	expected := pmetric.NewMetric()
	expected.SetName("system.cpu.time")
	expected.SetDescription("Total seconds each logical CPU spent on each mode.")
	expected.SetUnit("s")
	expected.SetEmptySum()
	internal.AssertDescriptorEqual(t, expected, metric)
	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, metric, startTime)
	}
	assert.GreaterOrEqual(t, metric.Sum().DataPoints().Len(), 4*runtime.NumCPU())
	internal.AssertSumMetricHasAttribute(t, metric, 0, "cpu")
	internal.AssertSumMetricHasAttributeValue(t, metric, 0, "state",
		pcommon.NewValueStr(metadata.AttributeStateUser.String()))
	internal.AssertSumMetricHasAttributeValue(t, metric, 1, "state",
		pcommon.NewValueStr(metadata.AttributeStateSystem.String()))
	internal.AssertSumMetricHasAttributeValue(t, metric, 2, "state",
		pcommon.NewValueStr(metadata.AttributeStateIdle.String()))
	internal.AssertSumMetricHasAttributeValue(t, metric, 3, "state",
		pcommon.NewValueStr(metadata.AttributeStateInterrupt.String()))
}

func assertCPUMetricHasLinuxSpecificStateLabels(t *testing.T, metric pmetric.Metric) {
	internal.AssertSumMetricHasAttributeValue(t, metric, 4, "state",
		pcommon.NewValueStr(metadata.AttributeStateNice.String()))
	internal.AssertSumMetricHasAttributeValue(t, metric, 5, "state",
		pcommon.NewValueStr(metadata.AttributeStateSoftirq.String()))
	internal.AssertSumMetricHasAttributeValue(t, metric, 6, "state",
		pcommon.NewValueStr(metadata.AttributeStateSteal.String()))
	internal.AssertSumMetricHasAttributeValue(t, metric, 7, "state",
		pcommon.NewValueStr(metadata.AttributeStateWait.String()))
}

func assertCPULogicalCountMetricValid(t *testing.T, metric pmetric.Metric) {
	expected := pmetric.NewMetric()
	expected.SetName("system.cpu.logical.count")
	expected.SetDescription("Number of available logical CPUs.")
	expected.SetUnit("{cpu}")
	expected.SetEmptySum()
	internal.AssertDescriptorEqual(t, expected, metric)

	require.False(t, metric.Sum().IsMonotonic())
	require.Equal(t, pmetric.AggregationTemporalityCumulative, metric.Sum().AggregationTemporality())
	require.Equal(t, 1, metric.Sum().DataPoints().Len())
	dataPoint := metric.Sum().DataPoints().At(0)
	require.Positive(t, dataPoint.IntValue(), "Logical CPU count should be greater than 0")
}

func assertCPUPhysicalCountMetricValid(t *testing.T, metric pmetric.Metric) {
	expected := pmetric.NewMetric()
	expected.SetName("system.cpu.physical.count")
	expected.SetDescription("Number of available physical CPUs.")
	expected.SetUnit("{cpu}")
	expected.SetEmptySum()
	internal.AssertDescriptorEqual(t, expected, metric)

	require.False(t, metric.Sum().IsMonotonic())
	require.Equal(t, pmetric.AggregationTemporalityCumulative, metric.Sum().AggregationTemporality())
	require.Equal(t, 1, metric.Sum().DataPoints().Len())
	dataPoint := metric.Sum().DataPoints().At(0)
	require.Positive(t, dataPoint.IntValue(), "Physical CPU count should be greater than 0")
}

func assertCPUUtilizationMetricValid(t *testing.T, metric pmetric.Metric, startTime pcommon.Timestamp) {
	expected := pmetric.NewMetric()
	expected.SetName("system.cpu.utilization")
	expected.SetDescription("Difference in system.cpu.time since the last measurement per logical CPU, divided by the elapsed time (value in interval [0,1]).")
	expected.SetUnit("1")
	expected.SetEmptyGauge()
	internal.AssertDescriptorEqual(t, expected, metric)
	if startTime != 0 {
		internal.AssertGaugeMetricStartTimeEquals(t, metric, startTime)
	}
	internal.AssertGaugeMetricHasAttribute(t, metric, 0, "cpu")
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 0, "state",
		pcommon.NewValueStr(metadata.AttributeStateUser.String()))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 1, "state",
		pcommon.NewValueStr(metadata.AttributeStateSystem.String()))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 2, "state",
		pcommon.NewValueStr(metadata.AttributeStateIdle.String()))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 3, "state",
		pcommon.NewValueStr(metadata.AttributeStateInterrupt.String()))
}

func assertCPUUtilizationMetricHasLinuxSpecificStateLabels(t *testing.T, metric pmetric.Metric) {
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 4, "state",
		pcommon.NewValueStr(metadata.AttributeStateNice.String()))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 5, "state",
		pcommon.NewValueStr(metadata.AttributeStateSoftirq.String()))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 6, "state",
		pcommon.NewValueStr(metadata.AttributeStateSteal.String()))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 7, "state",
		pcommon.NewValueStr(metadata.AttributeStateWait.String()))
}
