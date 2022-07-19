// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadataparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestDataType(t *testing.T) {
	testCases := map[string]struct {
		dataType         MetricDataType
		expectedDataType pmetric.MetricDataType
		expectError      bool
	}{
		"Gauge":   {GaugeMetricDataType, pmetric.MetricDataTypeGauge, false},
		"Sum":     {SumMetricDataType, pmetric.MetricDataTypeSum, false},
		"Invalid": {UnknownMetricDataType, pmetric.MetricDataTypeNone, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			metricType := MetricType{
				DataType: testCase.dataType,
			}

			actualDataType, err := metricType.dataType()

			if testCase.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, testCase.expectedDataType, actualDataType)
		})
	}
}

func TestAggregationTemporality(t *testing.T) {
	testCases := map[string]struct {
		aggregationTemporality         AggregationType
		expectedAggregationTemporality pmetric.MetricAggregationTemporality
		expectError                    bool
	}{
		"Cumulative": {CumulativeAggregationType, pmetric.MetricAggregationTemporalityCumulative, false},
		"Delta":      {DeltaAggregationType, pmetric.MetricAggregationTemporalityDelta, false},
		"Empty":      {"", pmetric.MetricAggregationTemporalityUnspecified, false},
		"Invalid":    {UnknownAggregationType, pmetric.MetricAggregationTemporalityUnspecified, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			metricType := MetricType{
				Aggregation: testCase.aggregationTemporality,
			}

			actualAggregationTemporality, err := metricType.aggregationTemporality()

			if testCase.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, testCase.expectedAggregationTemporality, actualAggregationTemporality)
		})
	}
}

func TestToMetricDataType(t *testing.T) {
	testCases := map[string]struct {
		dataType                       MetricDataType
		aggregationTemporality         AggregationType
		expectedDataType               pmetric.MetricDataType
		expectedAggregationTemporality pmetric.MetricAggregationTemporality
		isMonotonic                    bool
		expectError                    bool
	}{
		"Happy path":          {GaugeMetricDataType, CumulativeAggregationType, pmetric.MetricDataTypeGauge, pmetric.MetricAggregationTemporalityCumulative, true, false},
		"Invalid data type":   {"invalid", CumulativeAggregationType, pmetric.MetricDataTypeNone, pmetric.MetricAggregationTemporalityCumulative, true, true},
		"Invalid aggregation": {GaugeMetricDataType, "invalid", pmetric.MetricDataTypeGauge, pmetric.MetricAggregationTemporalityUnspecified, true, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			metricType := MetricType{
				DataType:    testCase.dataType,
				Aggregation: testCase.aggregationTemporality,
				Monotonic:   testCase.isMonotonic,
			}

			metricDataType, err := metricType.toMetricDataType()

			if testCase.expectError {
				require.Error(t, err)
				require.Nil(t, metricDataType)
			} else {
				require.NoError(t, err)
				require.NotNil(t, metricDataType)
				assert.Equal(t, testCase.expectedDataType, metricDataType.MetricDataType())
				assert.Equal(t, testCase.expectedAggregationTemporality, metricDataType.AggregationTemporality())
				assert.Equal(t, testCase.isMonotonic, metricDataType.IsMonotonic())
			}
		})
	}
}
