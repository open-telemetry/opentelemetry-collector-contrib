// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadataparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

const (
	metricName       = "labelName"
	metricColumnName = "labelColumnName"
	metricUnit       = "metricUnit"
)

func TestMetric_ToMetricValueMetadata(t *testing.T) {
	testCases := map[string]struct {
		valueType        metadata.ValueType
		dataType         MetricType
		expectedDataType pmetric.MetricType
		expectError      bool
	}{
		"Value type is int and data type is gauge":     {metadata.IntValueType, MetricType{DataType: GaugeMetricDataType}, pmetric.MetricTypeGauge, false},
		"Value type is int and data type is sum":       {metadata.IntValueType, MetricType{DataType: SumMetricDataType, Aggregation: DeltaAggregationType, Monotonic: true}, pmetric.MetricTypeSum, false},
		"Value type is int and data type is unknown":   {metadata.IntValueType, MetricType{DataType: UnknownMetricDataType}, pmetric.MetricTypeEmpty, true},
		"Value type is float and data type is gauge":   {metadata.FloatValueType, MetricType{DataType: GaugeMetricDataType}, pmetric.MetricTypeGauge, false},
		"Value type is float and data type is sum":     {metadata.FloatValueType, MetricType{DataType: SumMetricDataType, Aggregation: DeltaAggregationType, Monotonic: true}, pmetric.MetricTypeSum, false},
		"Value type is float and data type is unknown": {metadata.FloatValueType, MetricType{DataType: UnknownMetricDataType}, pmetric.MetricTypeEmpty, true},
		"Value type is unknown and data type is gauge": {metadata.UnknownValueType, MetricType{DataType: GaugeMetricDataType}, pmetric.MetricTypeEmpty, true},
		"Value type is unknown and data type is sum":   {metadata.UnknownValueType, MetricType{DataType: SumMetricDataType, Aggregation: DeltaAggregationType, Monotonic: true}, pmetric.MetricTypeEmpty, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			metric := Metric{
				Label: Label{
					Name:       metricName,
					ColumnName: metricColumnName,
					ValueType:  testCase.valueType,
				},
				DataType: testCase.dataType,
				Unit:     metricUnit,
			}

			valueMetadata, err := metric.toMetricValueMetadata()

			if testCase.expectError {
				require.Nil(t, valueMetadata)
				require.Error(t, err)
			} else {
				require.NotNil(t, valueMetadata)
				require.NoError(t, err)

				assert.Equal(t, metric.Name, valueMetadata.Name())
				assert.Equal(t, metric.ColumnName, valueMetadata.ColumnName())
				assert.Equal(t, metric.Unit, valueMetadata.Unit())
				assert.Equal(t, testCase.expectedDataType, valueMetadata.DataType().MetricType())
				assert.Equal(t, metric.ValueType, valueMetadata.ValueType())
			}
		})
	}
}
