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

package memoryscraper

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/shirou/gopsutil/v3/mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	type testCase struct {
		name              string
		config            Config
		bootTimeFunc      func() (uint64, error)
		virtualMemoryFunc func() (*mem.VirtualMemoryStat, error)
		initializationErr string
		scrapeErr         string
		expectMetrics     int
		expectedStartTime pdata.Timestamp
	}

	testCases := []testCase{
		{
			name:          "Standard",
			config:        Config{Metrics: metadata.DefaultMetricsSettings()},
			expectMetrics: metricsLen,
		},
		{
			name:              "Validate Start Time",
			config:            Config{Metrics: metadata.DefaultMetricsSettings()},
			bootTimeFunc:      func() (uint64, error) { return 100, nil },
			expectMetrics:     metricsLen,
			expectedStartTime: 100 * 1e9,
		},
		{
			name:              "Boot Time Error",
			config:            Config{Metrics: metadata.DefaultMetricsSettings()},
			bootTimeFunc:      func() (uint64, error) { return 0, errors.New("err1") },
			initializationErr: "err1",
			expectMetrics:     metricsLen,
		},
		{
			name: "Disable one metric",
			config: (func() Config {
				config := Config{Metrics: metadata.DefaultMetricsSettings()}
				config.Metrics.SystemMemoryUsage.Enabled = false
				return config
			})(),
			expectMetrics: metricsLen - 1,
		},
		{
			name:              "Error",
			config:            Config{Metrics: metadata.DefaultMetricsSettings()},
			virtualMemoryFunc: func() (*mem.VirtualMemoryStat, error) { return nil, errors.New("err1") },
			scrapeErr:         "err1",
			expectMetrics:     metricsLen,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			scraper := newMemoryScraper(context.Background(), &test.config)
			if test.bootTimeFunc != nil {
				scraper.bootTime = test.bootTimeFunc
			}
			if test.virtualMemoryFunc != nil {
				scraper.virtualMemory = test.virtualMemoryFunc
			}

			err := scraper.start(context.Background(), componenttest.NewNopHost())
			if test.initializationErr != "" {
				assert.EqualError(t, err, test.initializationErr)
				return
			}
			require.NoError(t, err, "Failed to initialize memory scraper: %v", err)

			md, err := scraper.scrape(context.Background())
			if test.scrapeErr != "" {
				assert.EqualError(t, err, test.scrapeErr)
				return
			}
			require.NoError(t, err, "Failed to scrape metrics: %v", err)

			assert.Equal(t, test.expectMetrics, md.MetricCount())
			metrics := md.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics()
			assert.Equal(t, test.expectMetrics, metrics.Len())

			if test.expectMetrics > 0 {
				assertMemoryUsageMetricValid(t, metrics.At(0), test.expectedStartTime)
				if runtime.GOOS == "linux" {
					assertMemoryUsageMetricHasLinuxSpecificStateLabels(t, metrics.At(0))
				} else if runtime.GOOS != "windows" {
					internal.AssertSumMetricHasAttributeValue(t, metrics.At(0), 2, metadata.Attributes.State, pdata.NewAttributeValueString(metadata.AttributeState.Inactive))
				}
				internal.AssertSameTimeStampForAllMetrics(t, metrics)
			}

		})
	}
}

func assertMemoryUsageMetricValid(t *testing.T, metric pdata.Metric, startTime pdata.Timestamp) {
	expected := pdata.NewMetric()
	expected.SetName("system.memory.usage")
	expected.SetDescription("Bytes of memory in use.")
	expected.SetUnit("By")
	expected.SetDataType(pdata.MetricDataTypeSum)
	internal.AssertDescriptorEqual(t, expected, metric)
	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, metric, startTime)
	}
	assert.GreaterOrEqual(t, metric.Sum().DataPoints().Len(), 2)
	internal.AssertSumMetricHasAttributeValue(t, metric, 0, metadata.Attributes.State, pdata.NewAttributeValueString(metadata.AttributeState.Used))
	internal.AssertSumMetricHasAttributeValue(t, metric, 1, metadata.Attributes.State, pdata.NewAttributeValueString(metadata.AttributeState.Free))
}

func assertMemoryUsageMetricHasLinuxSpecificStateLabels(t *testing.T, metric pdata.Metric) {
	internal.AssertSumMetricHasAttributeValue(t, metric, 2, metadata.Attributes.State, pdata.NewAttributeValueString(metadata.AttributeState.Buffered))
	internal.AssertSumMetricHasAttributeValue(t, metric, 3, metadata.Attributes.State, pdata.NewAttributeValueString(metadata.AttributeState.Cached))
	internal.AssertSumMetricHasAttributeValue(t, metric, 4, metadata.Attributes.State, pdata.NewAttributeValueString(metadata.AttributeState.SlabReclaimable))
	internal.AssertSumMetricHasAttributeValue(t, metric, 5, metadata.Attributes.State, pdata.NewAttributeValueString(metadata.AttributeState.SlabUnreclaimable))
}
