// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cpuscraper

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	type testCase struct {
		name                string
		bootTimeFunc        func() (uint64, error)
		timesFunc           func(bool) ([]cpu.TimesStat, error)
		metricsConfig       metadata.MetricsSettings
		expectedMetricCount int
		expectedStartTime   pdata.Timestamp
		initializationErr   string
		expectedErr         string
	}

	testCases := []testCase{
		{
			name:                "Standard",
			metricsConfig:       metadata.DefaultMetricsSettings(),
			expectedMetricCount: 1,
		},
		{
			name:                "Validate Start Time",
			bootTimeFunc:        func() (uint64, error) { return 100, nil },
			metricsConfig:       metadata.DefaultMetricsSettings(),
			expectedMetricCount: 1,
			expectedStartTime:   100 * 1e9,
		},
		{
			name:                "Boot Time Error",
			bootTimeFunc:        func() (uint64, error) { return 0, errors.New("err1") },
			metricsConfig:       metadata.DefaultMetricsSettings(),
			expectedMetricCount: 1,
			initializationErr:   "err1",
		},
		{
			name:                "Times Error",
			timesFunc:           func(bool) ([]cpu.TimesStat, error) { return nil, errors.New("err2") },
			metricsConfig:       metadata.DefaultMetricsSettings(),
			expectedMetricCount: 1,
			expectedErr:         "err2",
		},
		{
			name: "SystemCPUTime metric is disabled ",
			metricsConfig: metadata.MetricsSettings{
				SystemCPUTime: metadata.MetricSettings{
					Enabled: false,
				},
			},
			expectedMetricCount: 0,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper := newCPUScraper(context.Background(), &Config{Metrics: test.metricsConfig})
			if test.bootTimeFunc != nil {
				scraper.bootTime = test.bootTimeFunc
			}
			if test.timesFunc != nil {
				scraper.times = test.timesFunc
			}

			err := scraper.start(context.Background(), componenttest.NewNopHost())
			if test.initializationErr != "" {
				assert.EqualError(t, err, test.initializationErr)
				return
			}
			require.NoError(t, err, "Failed to initialize cpu scraper: %v", err)

			md, err := scraper.scrape(context.Background())
			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)

				isPartial := scrapererror.IsPartialScrapeError(err)
				assert.True(t, isPartial)
				if isPartial {
					assert.Equal(t, 2, err.(scrapererror.PartialScrapeError).Failed)
				}

				return
			}
			require.NoError(t, err, "Failed to scrape metrics: %v", err)

			assert.Equal(t, test.expectedMetricCount, md.MetricCount())

			if test.expectedMetricCount > 0 {
				metrics := md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
				assertCPUMetricValid(t, metrics.At(0), test.expectedStartTime)

				if runtime.GOOS == "linux" {
					assertCPUMetricHasLinuxSpecificStateLabels(t, metrics.At(0))
				}

				internal.AssertSameTimeStampForAllMetrics(t, metrics)
			}
		})
	}
}

// TestScrape_CpuUtilization to test utilization we need to execute scrape at least twice to have
// data to calculate the difference, so assertions will be done after the second scraping
func TestScrape_CpuUtilization(t *testing.T) {
	type testCase struct {
		name                string
		metricsConfig       metadata.MetricsSettings
		expectedMetricCount int
		times               bool
		utilization         bool
		utilizationIndex    int
	}

	testCases := []testCase{
		{
			name:                "Standard",
			metricsConfig:       metadata.DefaultMetricsSettings(),
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
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			settings := test.metricsConfig
			if test.metricsConfig == (metadata.MetricsSettings{}) {
				settings = metadata.MetricsSettings{
					SystemCPUTime: metadata.MetricSettings{
						Enabled: test.times,
					},
					SystemCPUUtilization: metadata.MetricSettings{
						Enabled: test.utilization,
					},
				}
			}

			scraper := newCPUScraper(context.Background(), &Config{Metrics: settings})
			err := scraper.start(context.Background(), componenttest.NewNopHost())
			require.NoError(t, err, "Failed to initialize cpu scraper: %v", err)

			_, err = scraper.scrape(context.Background())
			require.NoError(t, err, "Failed to scrape metrics: %v", err)
			//2nd scrape will trigger utilization metrics calculation
			md, err := scraper.scrape(context.Background())
			require.NoError(t, err, "Failed to scrape metrics: %v", err)

			assert.Equal(t, test.expectedMetricCount, md.MetricCount())
			metrics := md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
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

func TestScrape_CpuUtilizationError(t *testing.T) {
	scraper := newCPUScraper(context.Background(), &Config{Metrics: metadata.DefaultMetricsSettings()})
	err := scraper.start(context.Background(), componenttest.NewNopHost())
	require.NoError(t, err, "Failed to initialize cpu scraper: %v", err)

	scraper.now = func() time.Time {
		now, _ := time.Parse(time.RFC3339, "2021-12-21 00:23:21")
		return now
	}

	_, err = scraper.scrape(context.Background())
	require.NoError(t, err, "Failed to scrape metrics: %v", err)
	//2nd scrape will trigger utilization metrics calculation
	md, err := scraper.scrape(context.Background())
	var partialScrapeErr scrapererror.PartialScrapeError
	assert.ErrorAs(t, err, &partialScrapeErr)
	assert.Equal(t, 0, md.MetricCount())
}

func TestScrape_CpuUtilizationStandard(t *testing.T) {
	metricSettings := metadata.MetricsSettings{
		SystemCPUUtilization: metadata.MetricSettings{
			Enabled: true,
		},
	}

	//datapoint data
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

	cpuScraper := newCPUScraper(context.Background(), &Config{Metrics: metricSettings})
	for _, scrapeData := range scrapesData {
		//mock TimeStats and Now
		cpuScraper.times = func(_ bool) ([]cpu.TimesStat, error) {
			return scrapeData.times, nil
		}
		cpuScraper.now = func() time.Time {
			now, _ := time.Parse(time.RFC3339, scrapeData.scrapeTime)
			return now
		}

		err := cpuScraper.start(context.Background(), componenttest.NewNopHost())
		require.NoError(t, err, "Failed to initialize cpu scraper: %v", err)

		md, err := cpuScraper.scrape(context.Background())
		require.NoError(t, err)
		//no metrics in the first scrape
		if len(scrapeData.expectedDps) == 0 {
			assert.Equal(t, 0, md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics().Len())
			continue
		}

		assert.Equal(t, 1, md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics().Len())
		metric := md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics().At(0)
		assertCPUUtilizationMetricValid(t, metric, 0)
		dp := metric.Gauge().DataPoints()

		expectedDataPoints := 8
		if runtime.GOOS == "linux" {
			expectedDataPoints = 16
			assertCPUUtilizationMetricHasLinuxSpecificStateLabels(t, metric)
		}
		assert.Equal(t, expectedDataPoints, dp.Len())

		//remove empty values to make the test more simple
		dp.RemoveIf(func(n pdata.NumberDataPoint) bool {
			return n.DoubleVal() == 0.0
		})

		for idx, expectedDp := range scrapeData.expectedDps {
			assertDatapointValueAndStringAttributes(t, dp.At(idx), expectedDp.val, expectedDp.attrs)
		}
	}
}

func assertDatapointValueAndStringAttributes(t *testing.T, dp pdata.NumberDataPoint, value float64, attrs map[string]string) {
	assert.InDelta(t, value, dp.DoubleVal(), 0.0001)
	for k, v := range attrs {
		cpuAttribute, exists := dp.Attributes().Get(k)
		assert.True(t, exists)
		assert.Equal(t, v, cpuAttribute.StringVal())
	}
}

func assertCPUMetricValid(t *testing.T, metric pdata.Metric, startTime pdata.Timestamp) {
	expected := pdata.NewMetric()
	expected.SetName("system.cpu.time")
	expected.SetDescription("Total CPU seconds broken down by different states.")
	expected.SetUnit("s")
	expected.SetDataType(pdata.MetricDataTypeSum)
	internal.AssertDescriptorEqual(t, expected, metric)
	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, metric, startTime)
	}
	assert.GreaterOrEqual(t, metric.Sum().DataPoints().Len(), 4*runtime.NumCPU())
	internal.AssertSumMetricHasAttribute(t, metric, 0, metadata.Attributes.Cpu)
	internal.AssertSumMetricHasAttributeValue(t, metric, 0, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.User))
	internal.AssertSumMetricHasAttributeValue(t, metric, 1, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.System))
	internal.AssertSumMetricHasAttributeValue(t, metric, 2, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.Idle))
	internal.AssertSumMetricHasAttributeValue(t, metric, 3, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.Interrupt))
}

func assertCPUMetricHasLinuxSpecificStateLabels(t *testing.T, metric pdata.Metric) {
	internal.AssertSumMetricHasAttributeValue(t, metric, 4, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.Nice))
	internal.AssertSumMetricHasAttributeValue(t, metric, 5, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.Softirq))
	internal.AssertSumMetricHasAttributeValue(t, metric, 6, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.Steal))
	internal.AssertSumMetricHasAttributeValue(t, metric, 7, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.Wait))
}

func assertCPUUtilizationMetricValid(t *testing.T, metric pdata.Metric, startTime pdata.Timestamp) {
	expected := pdata.NewMetric()
	expected.SetName("system.cpu.utilization")
	expected.SetDescription("Percentage of CPU time broken down by different states.")
	expected.SetUnit("1")
	expected.SetDataType(pdata.MetricDataTypeGauge)
	internal.AssertDescriptorEqual(t, expected, metric)
	if startTime != 0 {
		internal.AssertGaugeMetricStartTimeEquals(t, metric, startTime)
	}
	internal.AssertGaugeMetricHasAttribute(t, metric, 0, metadata.Attributes.Cpu)
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 0, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.User))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 1, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.System))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 2, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.Idle))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 3, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.Interrupt))
}

func assertCPUUtilizationMetricHasLinuxSpecificStateLabels(t *testing.T, metric pdata.Metric) {
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 4, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.Nice))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 5, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.Softirq))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 6, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.Steal))
	internal.AssertGaugeMetricHasAttributeValue(t, metric, 7, metadata.Attributes.State, pdata.NewValueString(metadata.AttributeState.Wait))
}
