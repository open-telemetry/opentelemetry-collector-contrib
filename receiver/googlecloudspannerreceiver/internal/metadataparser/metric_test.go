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
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

const (
	metricName       = "labelName"
	metricColumnName = "labelColumnName"
	metricUnit       = "metricUnit"
)

func TestMetric_ToMetricValueMetadata(t *testing.T) {
	testCases := map[string]struct {
		valueType        string
		dataType         MetricType
		expectedType     interface{}
		expectedDataType pdata.MetricDataType
		expectError      bool
	}{
		"Value type is int and data type is gauge":     {metricValueTypeInt, MetricType{DataType: metricDataTypeGauge}, metadata.Int64MetricValueMetadata{}, pdata.MetricDataTypeGauge, false},
		"Value type is int and data type is sum":       {metricValueTypeInt, MetricType{DataType: metricDataTypeSum, Aggregation: aggregationTemporalityDelta, Monotonic: true}, metadata.Int64MetricValueMetadata{}, pdata.MetricDataTypeSum, false},
		"Value type is int and data type is unknown":   {metricValueTypeInt, MetricType{DataType: "unknown"}, nil, pdata.MetricDataTypeNone, true},
		"Value type is float and data type is gauge":   {metricValueTypeFloat, MetricType{DataType: metricDataTypeGauge}, metadata.Float64MetricValueMetadata{}, pdata.MetricDataTypeGauge, false},
		"Value type is float and data type is sum":     {metricValueTypeFloat, MetricType{DataType: metricDataTypeSum, Aggregation: aggregationTemporalityDelta, Monotonic: true}, metadata.Float64MetricValueMetadata{}, pdata.MetricDataTypeSum, false},
		"Value type is float and data type is unknown": {metricValueTypeFloat, MetricType{DataType: "unknown"}, nil, pdata.MetricDataTypeNone, true},
		"Value type is unknown and data type is gauge": {"unknown", MetricType{DataType: metricDataTypeGauge}, nil, pdata.MetricDataTypeNone, true},
		"Value type is unknown and data type is sum":   {"unknown", MetricType{DataType: metricDataTypeSum, Aggregation: aggregationTemporalityDelta, Monotonic: true}, nil, pdata.MetricDataTypeNone, true},
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

				assert.IsType(t, testCase.expectedType, valueMetadata)
				assert.Equal(t, metric.Name, valueMetadata.Name())
				assert.Equal(t, metric.ColumnName, valueMetadata.ColumnName())
				assert.Equal(t, metric.Unit, valueMetadata.Unit())
				assert.Equal(t, testCase.expectedDataType, valueMetadata.DataType().MetricDataType())
			}
		})
	}
}
