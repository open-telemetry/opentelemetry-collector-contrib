// Copyright The OpenTelemetry Authors
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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/metadata"
)

func TestMetadata_ToLabelValuesMetadata(t *testing.T) {
	testCases := map[string]struct {
		valueType   metadata.ValueType
		expectError bool
	}{
		"Happy path": {metadata.StringValueType, false},
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
		valueType   metadata.ValueType
		dataType    MetricType
		expectError bool
	}{
		"Happy path": {metadata.IntValueType, MetricType{DataType: GaugeMetricDataType}, false},
		"With error": {metadata.UnknownValueType, MetricType{DataType: GaugeMetricDataType}, true},
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
		labelValueType  metadata.ValueType
		metricValueType metadata.ValueType
		dataType        MetricType
		expectError     bool
	}{
		"Happy path":        {metadata.IntValueType, metadata.IntValueType, MetricType{DataType: GaugeMetricDataType}, false},
		"With label error":  {metadata.UnknownValueType, metadata.IntValueType, MetricType{DataType: GaugeMetricDataType}, true},
		"With metric error": {metadata.IntValueType, metadata.UnknownValueType, MetricType{DataType: GaugeMetricDataType}, true},
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
				HighCardinality:     true,
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
				assert.Equal(t, md.HighCardinality, metricsMetadata.HighCardinality)
				assert.Equal(t, 1, len(metricsMetadata.QueryLabelValuesMetadata))
				assert.Equal(t, 1, len(metricsMetadata.QueryMetricValuesMetadata))
			}
		})
	}
}
