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
)

func TestMetadata_ToLabelValuesMetadata(t *testing.T) {
	testCases := map[string]struct {
		valueType   string
		expectError bool
	}{
		"Happy path": {labelValueTypeString, false},
		"With error": {"unknown", true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			label := Label{
				Name:       labelName,
				ColumnName: labelColumnName,
				ValueType:  testCase.valueType,
			}
			md := Metadata{
				Labels: []Label{label},
			}

			valuesMetadata, err := md.toLabelValuesMetadata()

			if testCase.expectError {
				require.Nil(t, valuesMetadata)
				require.Error(t, err)
			} else {
				require.NotNil(t, valuesMetadata)
				require.NoError(t, err)

				assert.Equal(t, 1, len(valuesMetadata))
			}
		})
	}
}

func TestMetadata_ToMetricValuesMetadata(t *testing.T) {
	testCases := map[string]struct {
		valueType   string
		dataType    MetricType
		expectError bool
	}{
		"Happy path": {metricValueTypeInt, MetricType{DataType: metricDataTypeGauge}, false},
		"With error": {"unknown", MetricType{DataType: metricDataTypeGauge}, true},
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
			}
			md := Metadata{
				Metrics: []Metric{metric},
			}

			valuesMetadata, err := md.toMetricValuesMetadata()

			if testCase.expectError {
				require.Nil(t, valuesMetadata)
				require.Error(t, err)
			} else {
				require.NotNil(t, valuesMetadata)
				require.NoError(t, err)

				assert.Equal(t, 1, len(valuesMetadata))
			}
		})
	}
}

func TestMetadata_MetricsMetadata(t *testing.T) {
	testCases := map[string]struct {
		labelValueType  string
		metricValueType string
		dataType        MetricType
		expectError     bool
	}{
		"Happy path":        {labelValueTypeInt, metricValueTypeInt, MetricType{DataType: metricDataTypeGauge}, false},
		"With label error":  {"unknown", metricValueTypeInt, MetricType{DataType: metricDataTypeGauge}, true},
		"With metric error": {labelValueTypeInt, "unknown", MetricType{DataType: metricDataTypeGauge}, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			label := Label{
				Name:       labelName,
				ColumnName: labelColumnName,
				ValueType:  testCase.labelValueType,
			}
			metric := Metric{
				Label: Label{
					Name:       metricName,
					ColumnName: metricColumnName,
					ValueType:  testCase.metricValueType,
				},
				DataType: testCase.dataType,
			}
			md := Metadata{
				Name:                "name",
				Query:               "query",
				MetricNamePrefix:    "metricNamePrefix",
				TimestampColumnName: "timestampColumnName",
				Labels:              []Label{label},
				Metrics:             []Metric{metric},
			}

			metricsMetadata, err := md.MetricsMetadata()

			if testCase.expectError {
				require.Nil(t, metricsMetadata)
				require.Error(t, err)
			} else {
				require.NotNil(t, metricsMetadata)
				require.NoError(t, err)

				assert.Equal(t, md.Name, metricsMetadata.Name)
				assert.Equal(t, md.Query, metricsMetadata.Query)
				assert.Equal(t, md.MetricNamePrefix, metricsMetadata.MetricNamePrefix)
				assert.Equal(t, md.TimestampColumnName, metricsMetadata.TimestampColumnName)
				assert.Equal(t, 1, len(metricsMetadata.QueryLabelValuesMetadata))
				assert.Equal(t, 1, len(metricsMetadata.QueryMetricValuesMetadata))
			}
		})
	}
}
