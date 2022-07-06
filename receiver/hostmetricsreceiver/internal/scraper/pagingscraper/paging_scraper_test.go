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

package pagingscraper

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/service/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper/internal/metadata"
)

func TestScrape(t *testing.T) {
	type testCase struct {
		name                                       string
		config                                     Config
		bootTimeFunc                               func() (uint64, error)
		expectedStartTime                          pcommon.Timestamp
		initializationErr                          string
		removeDirectionAttributeFeatureGateEnabled bool
	}

	config := metadata.DefaultMetricsSettings()
	config.SystemPagingUtilization.Enabled = true

	testCases := []testCase{
		{
			name:   "Standard",
			config: Config{Metrics: config},
		},
		{
			name:   "Standard with direction removed",
			config: Config{Metrics: config},
			removeDirectionAttributeFeatureGateEnabled: true,
		},
		{
			name:              "Validate Start Time",
			config:            Config{Metrics: config},
			bootTimeFunc:      func() (uint64, error) { return 100, nil },
			expectedStartTime: 100 * 1e9,
		},
		{
			name:              "Boot Time Error",
			config:            Config{Metrics: config},
			bootTimeFunc:      func() (uint64, error) { return 0, errors.New("err1") },
			initializationErr: "err1",
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			featuregate.GetRegistry().Apply(map[string]bool{removeDirectionAttributeFeatureGateID: test.removeDirectionAttributeFeatureGateEnabled})
			scraper := newPagingScraper(context.Background(), componenttest.NewNopReceiverCreateSettings(), &test.config)
			if test.bootTimeFunc != nil {
				scraper.bootTime = test.bootTimeFunc
			}

			err := scraper.start(context.Background(), componenttest.NewNopHost())
			if test.initializationErr != "" {
				assert.EqualError(t, err, test.initializationErr)
				return
			}
			require.NoError(t, err, "Failed to initialize paging scraper: %v", err)

			md, err := scraper.scrape(context.Background())
			require.NoError(t, err)
			metrics := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics()

			// Windows does not currently support the faults metric
			expectedMetrics := 4
			if runtime.GOOS == "windows" {
				expectedMetrics = 3
			}
			if test.removeDirectionAttributeFeatureGateEnabled {
				// in/out are separated into an additional metric
				expectedMetrics++
			}

			assert.Equal(t, expectedMetrics, md.MetricCount())

			startIndex := 0
			if runtime.GOOS != "windows" {
				assertPageFaultsMetricValid(t, metrics.At(startIndex), test.expectedStartTime)
				startIndex++
			}

			if test.removeDirectionAttributeFeatureGateEnabled {
				assertPagingOperationsMetricValid(t, []pmetric.Metric{metrics.At(startIndex), metrics.At(startIndex + 1)}, test.expectedStartTime, test.removeDirectionAttributeFeatureGateEnabled)
				startIndex++
			} else {
				assertPagingOperationsMetricValid(t, []pmetric.Metric{metrics.At(startIndex)}, test.expectedStartTime, test.removeDirectionAttributeFeatureGateEnabled)
			}

			internal.AssertSameTimeStampForMetrics(t, metrics, 0, metrics.Len()-2)
			startIndex++

			assertPagingUsageMetricValid(t, metrics.At(startIndex))
			internal.AssertSameTimeStampForMetrics(t, metrics, startIndex, metrics.Len())
			startIndex++
			assertPagingUtilizationMetricValid(t, metrics.At(startIndex))
		})
	}
}

func assertPagingUsageMetricValid(t *testing.T, hostPagingUsageMetric pmetric.Metric) {
	expected := pmetric.NewMetric()
	expected.SetName("system.paging.usage")
	expected.SetDescription("Swap (unix) or pagefile (windows) usage.")
	expected.SetUnit("By")
	expected.SetDataType(pmetric.MetricDataTypeSum)
	internal.AssertDescriptorEqual(t, expected, hostPagingUsageMetric)

	// it's valid for a system to have no swap space  / paging file, so if no data points were returned, do no validation
	if hostPagingUsageMetric.Sum().DataPoints().Len() == 0 {
		return
	}

	// expect at least used, free & cached datapoint
	expectedDataPoints := 3
	// windows does not return a cached datapoint
	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		expectedDataPoints = 2
	}

	assert.GreaterOrEqual(t, hostPagingUsageMetric.Sum().DataPoints().Len(), expectedDataPoints)
	internal.AssertSumMetricHasAttributeValue(t, hostPagingUsageMetric, 0, "state",
		pcommon.NewValueString(metadata.AttributeStateUsed.String()))
	internal.AssertSumMetricHasAttributeValue(t, hostPagingUsageMetric, 1, "state",
		pcommon.NewValueString(metadata.AttributeStateFree.String()))
	// Windows and Linux do not support cached state label
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		internal.AssertSumMetricHasAttributeValue(t, hostPagingUsageMetric, 2, "state",
			pcommon.NewValueString(metadata.AttributeStateCached.String()))
	}

	// on Windows and Linux, also expect the page file device name label
	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		internal.AssertSumMetricHasAttribute(t, hostPagingUsageMetric, 0, "device")
		internal.AssertSumMetricHasAttribute(t, hostPagingUsageMetric, 1, "device")
	}
}

func assertPagingUtilizationMetricValid(t *testing.T, hostPagingUtilizationMetric pmetric.Metric) {
	expected := pmetric.NewMetric()
	expected.SetName("system.paging.utilization")
	expected.SetDescription("Swap (unix) or pagefile (windows) utilization.")
	expected.SetUnit("1")
	expected.SetDataType(pmetric.MetricDataTypeGauge)
	internal.AssertDescriptorEqual(t, expected, hostPagingUtilizationMetric)

	// it's valid for a system to have no swap space  / paging file, so if no data points were returned, do no validation
	if hostPagingUtilizationMetric.Gauge().DataPoints().Len() == 0 {
		return
	}

	// expect at least used, free & cached datapoint
	expectedDataPoints := 3
	// Windows does not return a cached datapoint
	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		expectedDataPoints = 2
	}

	assert.GreaterOrEqual(t, hostPagingUtilizationMetric.Gauge().DataPoints().Len(), expectedDataPoints)
	internal.AssertGaugeMetricHasAttributeValue(t, hostPagingUtilizationMetric, 0, "state",
		pcommon.NewValueString(metadata.AttributeStateUsed.String()))
	internal.AssertGaugeMetricHasAttributeValue(t, hostPagingUtilizationMetric, 1, "state",
		pcommon.NewValueString(metadata.AttributeStateFree.String()))
	// Windows and Linux do not support cached state label
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		internal.AssertGaugeMetricHasAttributeValue(t, hostPagingUtilizationMetric, 2, "state",
			pcommon.NewValueString(metadata.AttributeStateCached.String()))
	}

	// on Windows and Linux, also expect the page file device name label
	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		internal.AssertGaugeMetricHasAttribute(t, hostPagingUtilizationMetric, 0, "device")
		internal.AssertGaugeMetricHasAttribute(t, hostPagingUtilizationMetric, 1, "device")
	}
}

func assertPagingOperationsMetricValid(t *testing.T, pagingMetric []pmetric.Metric, startTime pcommon.Timestamp, removeAttribute bool) {

	type test struct {
		name        string
		description string
		unit        string
	}

	var tests []test

	if removeAttribute {
		tests = []test{
			{
				name:        "system.paging.operations.page_in",
				description: "The number of page_in operations.",
				unit:        "{operations}",
			},
			{
				name:        "system.paging.operations.page_out",
				description: "The number of page_out operations.",
				unit:        "{operations}",
			},
		}
	} else {
		tests = []test{
			{
				name:        "system.paging.operations",
				description: "The number of paging operations.",
				unit:        "{operations}",
			},
		}
	}

	for idx, tt := range tests {
		expected := pmetric.NewMetric()
		expected.SetName(tt.name)
		expected.SetDescription(tt.description)
		expected.SetUnit(tt.unit)
		expected.SetDataType(pmetric.MetricDataTypeSum)
		internal.AssertDescriptorEqual(t, expected, pagingMetric[idx])

		if startTime != 0 {
			internal.AssertSumMetricStartTimeEquals(t, pagingMetric[idx], startTime)
		}

		expectedDataPoints := 4
		if runtime.GOOS == "windows" {
			expectedDataPoints = 2
		}
		if removeAttribute {
			expectedDataPoints /= 2
		}

		assert.Equal(t, expectedDataPoints, pagingMetric[idx].Sum().DataPoints().Len())

		if removeAttribute {
			internal.AssertSumMetricHasAttributeValue(t, pagingMetric[idx], 0, "type",
				pcommon.NewValueString(metadata.AttributeTypeMajor.String()))
			if runtime.GOOS != "windows" {
				internal.AssertSumMetricHasAttributeValue(t, pagingMetric[idx], 1, "type",
					pcommon.NewValueString(metadata.AttributeTypeMinor.String()))
			}
		} else {
			internal.AssertSumMetricHasAttributeValue(t, pagingMetric[idx], 0, "type",
				pcommon.NewValueString(metadata.AttributeTypeMajor.String()))
			internal.AssertSumMetricHasAttributeValue(t, pagingMetric[idx], 0, "direction",
				pcommon.NewValueString(metadata.AttributeDirectionPageIn.String()))
			internal.AssertSumMetricHasAttributeValue(t, pagingMetric[idx], 1, "type",
				pcommon.NewValueString(metadata.AttributeTypeMajor.String()))
			internal.AssertSumMetricHasAttributeValue(t, pagingMetric[idx], 1, "direction",
				pcommon.NewValueString(metadata.AttributeDirectionPageOut.String()))
			if runtime.GOOS != "windows" {
				internal.AssertSumMetricHasAttributeValue(t, pagingMetric[idx], 2, "type",
					pcommon.NewValueString(metadata.AttributeTypeMinor.String()))
				internal.AssertSumMetricHasAttributeValue(t, pagingMetric[idx], 2, "direction",
					pcommon.NewValueString(metadata.AttributeDirectionPageIn.String()))
				internal.AssertSumMetricHasAttributeValue(t, pagingMetric[idx], 3, "type",
					pcommon.NewValueString(metadata.AttributeTypeMinor.String()))
				internal.AssertSumMetricHasAttributeValue(t, pagingMetric[idx], 3, "direction",
					pcommon.NewValueString(metadata.AttributeDirectionPageOut.String()))
			}
		}
	}
}

func assertPageFaultsMetricValid(t *testing.T, pageFaultsMetric pmetric.Metric, startTime pcommon.Timestamp) {
	expected := pmetric.NewMetric()
	expected.SetName("system.paging.faults")
	expected.SetDescription("The number of page faults.")
	expected.SetUnit("{faults}")
	expected.SetDataType(pmetric.MetricDataTypeSum)
	internal.AssertDescriptorEqual(t, expected, pageFaultsMetric)

	if startTime != 0 {
		internal.AssertSumMetricStartTimeEquals(t, pageFaultsMetric, startTime)
	}

	assert.Equal(t, 2, pageFaultsMetric.Sum().DataPoints().Len())
	internal.AssertSumMetricHasAttributeValue(t, pageFaultsMetric, 0, "type",
		pcommon.NewValueString(metadata.AttributeTypeMajor.String()))
	internal.AssertSumMetricHasAttributeValue(t, pageFaultsMetric, 1, "type",
		pcommon.NewValueString(metadata.AttributeTypeMinor.String()))
}
