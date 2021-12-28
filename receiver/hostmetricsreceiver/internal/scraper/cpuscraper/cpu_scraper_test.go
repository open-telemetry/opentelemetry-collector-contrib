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
					assert.Equal(t, 1, err.(scrapererror.PartialScrapeError).Failed)
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
	internal.AssertSumMetricHasAttributeValue(t, metric, 0, metadata.Attributes.State, pdata.NewAttributeValueString(metadata.AttributeState.User))
	internal.AssertSumMetricHasAttributeValue(t, metric, 1, metadata.Attributes.State, pdata.NewAttributeValueString(metadata.AttributeState.System))
	internal.AssertSumMetricHasAttributeValue(t, metric, 2, metadata.Attributes.State, pdata.NewAttributeValueString(metadata.AttributeState.Idle))
	internal.AssertSumMetricHasAttributeValue(t, metric, 3, metadata.Attributes.State, pdata.NewAttributeValueString(metadata.AttributeState.Interrupt))
}

func assertCPUMetricHasLinuxSpecificStateLabels(t *testing.T, metric pdata.Metric) {
	internal.AssertSumMetricHasAttributeValue(t, metric, 4, metadata.Attributes.State, pdata.NewAttributeValueString(metadata.AttributeState.Nice))
	internal.AssertSumMetricHasAttributeValue(t, metric, 5, metadata.Attributes.State, pdata.NewAttributeValueString(metadata.AttributeState.Softirq))
	internal.AssertSumMetricHasAttributeValue(t, metric, 6, metadata.Attributes.State, pdata.NewAttributeValueString(metadata.AttributeState.Steal))
	internal.AssertSumMetricHasAttributeValue(t, metric, 7, metadata.Attributes.State, pdata.NewAttributeValueString(metadata.AttributeState.Wait))
}
