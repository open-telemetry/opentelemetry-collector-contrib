// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
		expectedDataType pmetric.MetricType
		expectError      bool
	}{
		"Gauge":   {GaugeMetricDataType, pmetric.MetricTypeGauge, false},
		"Sum":     {SumMetricDataType, pmetric.MetricTypeSum, false},
		"Invalid": {UnknownMetricDataType, pmetric.MetricTypeEmpty, true},
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
		expectedAggregationTemporality pmetric.AggregationTemporality
		expectError                    bool
	}{
		"Cumulative": {CumulativeAggregationType, pmetric.AggregationTemporalityCumulative, false},
		"Delta":      {DeltaAggregationType, pmetric.AggregationTemporalityDelta, false},
		"Empty":      {"", pmetric.AggregationTemporalityUnspecified, false},
		"Invalid":    {UnknownAggregationType, pmetric.AggregationTemporalityUnspecified, true},
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

func TestToMetricType(t *testing.T) {
	testCases := map[string]struct {
		dataType                       MetricDataType
		aggregationTemporality         AggregationType
		expectedDataType               pmetric.MetricType
		expectedAggregationTemporality pmetric.AggregationTemporality
		isMonotonic                    bool
		expectError                    bool
	}{
		"Happy path":          {GaugeMetricDataType, CumulativeAggregationType, pmetric.MetricTypeGauge, pmetric.AggregationTemporalityCumulative, true, false},
		"Invalid data type":   {"invalid", CumulativeAggregationType, pmetric.MetricTypeEmpty, pmetric.AggregationTemporalityCumulative, true, true},
		"Invalid aggregation": {GaugeMetricDataType, "invalid", pmetric.MetricTypeGauge, pmetric.AggregationTemporalityUnspecified, true, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			metricType := MetricType{
				DataType:    testCase.dataType,
				Aggregation: testCase.aggregationTemporality,
				Monotonic:   testCase.isMonotonic,
			}

			metricDataType, err := metricType.toMetricType()

			if testCase.expectError {
				require.Error(t, err)
				require.Nil(t, metricDataType)
			} else {
				require.NoError(t, err)
				require.NotNil(t, metricDataType)
				assert.Equal(t, testCase.expectedDataType, metricDataType.MetricType())
				assert.Equal(t, testCase.expectedAggregationTemporality, metricDataType.AggregationTemporality())
				assert.Equal(t, testCase.isMonotonic, metricDataType.IsMonotonic())
			}
		})
	}
}
