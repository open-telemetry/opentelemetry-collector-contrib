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

package metadata

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/datasource"
)

func TestMetricsMetadata_ToMetrics(t *testing.T) {
	testCases := map[string]struct {
		metricsDataType pdata.MetricDataType
	}{
		"Gauge": {pdata.MetricDataTypeGauge},
		"Sum":   {pdata.MetricDataTypeSum},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			testMetricsMetadataToMetrics(t, testCase.metricsDataType)
		})
	}
}

func testMetricsMetadataToMetrics(t *testing.T, metricDataType pdata.MetricDataType) {
	timestamp := time.Now().UTC()
	labelValues := allPossibleLabelValues()
	metricValues := allPossibleMetricValues(metricDataType)
	databaseID := databaseID()
	metadata := MetricsMetadata{MetricNamePrefix: metricNamePrefix}

	metrics := metadata.toMetrics(databaseID, timestamp, labelValues, metricValues)

	assert.Equal(t, len(metricValues), len(metrics))

	for i, metric := range metrics {
		assert.Equal(t, 1, metric.DataPointCount())
		assert.Equal(t, 1, metric.MetricCount())
		assert.Equal(t, 1, metric.ResourceMetrics().Len())
		assert.Equal(t, 1, metric.ResourceMetrics().At(0).InstrumentationLibraryMetrics().Len())
		assert.Equal(t, 1, metric.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics().Len())

		ilMetric := metric.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics().At(0)

		assert.Equal(t, metadata.MetricNamePrefix+metricValues[i].Name(), ilMetric.Name())
		assert.Equal(t, metricValues[i].Unit(), ilMetric.Unit())
		assert.Equal(t, metricValues[i].DataType().MetricDataType(), ilMetric.DataType())

		var dataPoint pdata.NumberDataPoint

		if metricDataType == pdata.MetricDataTypeGauge {
			assert.NotNil(t, ilMetric.Gauge())
			assert.Equal(t, 1, ilMetric.Gauge().DataPoints().Len())
			dataPoint = ilMetric.Gauge().DataPoints().At(0)
		} else {
			assert.NotNil(t, ilMetric.Sum())
			assert.Equal(t, pdata.MetricAggregationTemporalityDelta, ilMetric.Sum().AggregationTemporality())
			assert.True(t, ilMetric.Sum().IsMonotonic())
			assert.Equal(t, 1, ilMetric.Sum().DataPoints().Len())
			dataPoint = ilMetric.Sum().DataPoints().At(0)
		}

		assertMetricValue(t, metricValues[i], dataPoint)

		assert.Equal(t, pdata.NewTimestampFromTime(timestamp), dataPoint.Timestamp())
		// Adding +3 here because we'll always have 3 labels added for each metric: project_id, instance_id, database
		assert.Equal(t, 3+len(labelValues), dataPoint.Attributes().Len())

		attributesMap := dataPoint.Attributes()

		assertDefaultLabels(t, attributesMap, databaseID)
		assertNonDefaultLabels(t, attributesMap, labelValues)
	}
}

func allPossibleLabelValues() []LabelValue {
	strLabelValue := stringLabelValue{
		StringLabelValueMetadata: StringLabelValueMetadata{
			queryLabelValueMetadata: newQueryLabelValueMetadata("stringLabelName", "stringLabelColumnName"),
		},
		value: stringValue,
	}
	bLabelValue := boolLabelValue{
		BoolLabelValueMetadata: BoolLabelValueMetadata{
			queryLabelValueMetadata: newQueryLabelValueMetadata("boolLabelName", "boolLabelColumnName"),
		},
		value: boolValue,
	}
	i64LabelValue := int64LabelValue{
		Int64LabelValueMetadata: Int64LabelValueMetadata{
			queryLabelValueMetadata: newQueryLabelValueMetadata("int64LabelName", "int64LabelColumnName"),
		},
		value: int64Value,
	}
	strSliceLabelValue := stringSliceLabelValue{
		StringSliceLabelValueMetadata: StringSliceLabelValueMetadata{
			queryLabelValueMetadata: newQueryLabelValueMetadata("stringSliceLabelName", "stringSliceLabelColumnName"),
		},
		value: stringValue,
	}
	btSliceLabelValue := byteSliceLabelValue{
		ByteSliceLabelValueMetadata: ByteSliceLabelValueMetadata{
			queryLabelValueMetadata: newQueryLabelValueMetadata("byteSliceLabelName", "byteSliceLabelColumnName"),
		},
		value: stringValue,
	}

	return []LabelValue{
		strLabelValue,
		bLabelValue,
		i64LabelValue,
		strSliceLabelValue,
		btSliceLabelValue,
	}
}

func allPossibleMetricValues(metricDataType pdata.MetricDataType) []MetricValue {
	dataType := NewMetricDataType(metricDataType, pdata.MetricAggregationTemporalityDelta, true)
	return []MetricValue{
		int64MetricValue{
			Int64MetricValueMetadata: Int64MetricValueMetadata{
				queryMetricValueMetadata: newQueryMetricValueMetadata("int64MetricName",
					"int64MetricColumnName", dataType, metricUnit),
			},
			value: int64Value,
		},
		float64MetricValue{
			Float64MetricValueMetadata: Float64MetricValueMetadata{
				queryMetricValueMetadata: newQueryMetricValueMetadata("float64MetricName",
					"float64MetricColumnName", dataType, metricUnit),
			},
			value: float64Value,
		},
	}
}

func assertDefaultLabels(t *testing.T, attributesMap pdata.AttributeMap, databaseID *datasource.DatabaseID) {
	assertStringLabelValue(t, attributesMap, projectIDLabelName, databaseID.ProjectID())
	assertStringLabelValue(t, attributesMap, instanceIDLabelName, databaseID.InstanceID())
	assertStringLabelValue(t, attributesMap, databaseLabelName, databaseID.DatabaseName())
}

func assertNonDefaultLabels(t *testing.T, attributesMap pdata.AttributeMap, labelValues []LabelValue) {
	for _, labelValue := range labelValues {
		assertLabelValue(t, attributesMap, labelValue)
	}
}

func assertLabelValue(t *testing.T, attributesMap pdata.AttributeMap, labelValue LabelValue) {
	value, exists := attributesMap.Get(labelValue.Name())

	assert.True(t, exists)
	switch labelValue.(type) {
	case stringLabelValue, stringSliceLabelValue, byteSliceLabelValue:
		assert.Equal(t, labelValue.Value(), value.StringVal())
	case boolLabelValue:
		assert.Equal(t, labelValue.Value(), value.BoolVal())
	case int64LabelValue:
		assert.Equal(t, labelValue.Value(), value.IntVal())
	default:
		assert.Fail(t, "Unknown label value type received")
	}
}

func assertStringLabelValue(t *testing.T, attributesMap pdata.AttributeMap, labelName string, expectedValue interface{}) {
	value, exists := attributesMap.Get(labelName)

	assert.True(t, exists)
	assert.Equal(t, expectedValue, value.StringVal())
}

func assertMetricValue(t *testing.T, metricValue MetricValue, dataPoint pdata.NumberDataPoint) {
	switch metricValue.(type) {
	case int64MetricValue:
		assert.Equal(t, metricValue.Value(), dataPoint.IntVal())
	case float64MetricValue:
		assert.Equal(t, metricValue.Value(), dataPoint.DoubleVal())
	}
}
