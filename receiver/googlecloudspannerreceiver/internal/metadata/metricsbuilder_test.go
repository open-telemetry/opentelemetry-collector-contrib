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
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/pdata"
)

const (
	metricName1 = "metricName1"
	metricName2 = "metricName2"
)

type testData struct {
	dataPoints           []*MetricsDataPoint
	expectedGroupingKeys []MetricsDataPointKey
	expectedGroups       map[MetricsDataPointKey][]*MetricsDataPoint
}

func TestMetricsBuilder_Build(t *testing.T) {
	testCases := map[string]struct {
		metricsDataType pdata.MetricDataType
	}{
		"Gauge": {pdata.MetricDataTypeGauge},
		"Sum":   {pdata.MetricDataTypeSum},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			testMetricsBuilderBuild(t, testCase.metricsDataType)
		})
	}
}

func testMetricsBuilderBuild(t *testing.T, metricDataType pdata.MetricDataType) {
	dataForTesting := generateTestData(metricDataType)
	metricsBuilder := &MetricsBuilder{}
	expectedGroupingKeysByMetricName := make(map[string]MetricsDataPointKey, len(dataForTesting.expectedGroupingKeys))

	for _, expectedGroupingKey := range dataForTesting.expectedGroupingKeys {
		expectedGroupingKeysByMetricName[expectedGroupingKey.MetricName] = expectedGroupingKey
	}

	metric := metricsBuilder.Build(dataForTesting.dataPoints)

	assert.Equal(t, len(dataForTesting.dataPoints), metric.DataPointCount())
	assert.Equal(t, len(dataForTesting.expectedGroups), metric.MetricCount())
	assert.Equal(t, 1, metric.ResourceMetrics().At(0).InstrumentationLibraryMetrics().Len())
	assert.Equal(t, len(dataForTesting.expectedGroups), metric.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics().Len())
	require.Equal(t, instrumentationLibraryName, metric.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).InstrumentationLibrary().Name())

	for i := 0; i < len(dataForTesting.expectedGroups); i++ {
		ilMetric := metric.ResourceMetrics().At(0).InstrumentationLibraryMetrics().At(0).Metrics().At(i)
		expectedGroupingKey := expectedGroupingKeysByMetricName[ilMetric.Name()]
		expectedDataPoints := dataForTesting.expectedGroups[expectedGroupingKey]

		for dataPointIndex, expectedDataPoint := range expectedDataPoints {
			assert.Equal(t, expectedDataPoint.metricName, ilMetric.Name())
			assert.Equal(t, expectedDataPoint.metricValue.Unit(), ilMetric.Unit())
			assert.Equal(t, expectedDataPoint.metricValue.DataType().MetricDataType(), ilMetric.DataType())

			var dataPoint pdata.NumberDataPoint

			if metricDataType == pdata.MetricDataTypeGauge {
				assert.NotNil(t, ilMetric.Gauge())
				assert.Equal(t, len(expectedDataPoints), ilMetric.Gauge().DataPoints().Len())
				dataPoint = ilMetric.Gauge().DataPoints().At(dataPointIndex)
			} else {
				assert.NotNil(t, ilMetric.Sum())
				assert.Equal(t, pdata.MetricAggregationTemporalityDelta, ilMetric.Sum().AggregationTemporality())
				assert.True(t, ilMetric.Sum().IsMonotonic())
				assert.Equal(t, len(expectedDataPoints), ilMetric.Sum().DataPoints().Len())
				dataPoint = ilMetric.Sum().DataPoints().At(dataPointIndex)
			}

			assertMetricValue(t, expectedDataPoint.metricValue, dataPoint)

			assert.Equal(t, pdata.NewTimestampFromTime(expectedDataPoint.timestamp), dataPoint.Timestamp())
			// Adding +3 here because we'll always have 3 labels added for each metric: project_id, instance_id, database
			assert.Equal(t, 3+len(expectedDataPoint.labelValues), dataPoint.Attributes().Len())

			attributesMap := dataPoint.Attributes()

			assertDefaultLabels(t, attributesMap, expectedDataPoint.databaseID)
			assertNonDefaultLabels(t, attributesMap, expectedDataPoint.labelValues)
		}
	}
}

func TestGroup(t *testing.T) {
	dataForTesting := generateTestData(metricDataType)

	groupedDataPoints := group(dataForTesting.dataPoints)

	assert.Equal(t, len(dataForTesting.expectedGroups), len(groupedDataPoints))

	for expectedGroupingKey, expectedGroupPoints := range dataForTesting.expectedGroups {
		dataPointsByKey := groupedDataPoints[expectedGroupingKey]

		assert.Equal(t, len(expectedGroupPoints), len(dataPointsByKey))

		for i, point := range expectedGroupPoints {
			assert.Equal(t, point, dataPointsByKey[i])
		}
	}
}

func TestGroup_NilDataPoints(t *testing.T) {
	groupedDataPoints := group(nil)

	assert.Equal(t, 0, len(groupedDataPoints))
}

func generateTestData(metricDataType pdata.MetricDataType) testData {
	timestamp1 := time.Now().UTC()
	timestamp2 := timestamp1.Add(time.Minute)
	labelValues := allPossibleLabelValues()
	metricValues := allPossibleMetricValues(metricDataType)

	dataPoints := []*MetricsDataPoint{
		newMetricDataPoint(metricName1, timestamp1, labelValues, metricValues[0]),
		newMetricDataPoint(metricName1, timestamp1, labelValues, metricValues[1]),
		newMetricDataPoint(metricName2, timestamp1, labelValues, metricValues[0]),
		newMetricDataPoint(metricName2, timestamp1, labelValues, metricValues[1]),
		newMetricDataPoint(metricName1, timestamp2, labelValues, metricValues[0]),
		newMetricDataPoint(metricName1, timestamp2, labelValues, metricValues[1]),
		newMetricDataPoint(metricName2, timestamp2, labelValues, metricValues[0]),
		newMetricDataPoint(metricName2, timestamp2, labelValues, metricValues[1]),
	}

	expectedGroupingKeys := []MetricsDataPointKey{
		{
			MetricName:     metricName1,
			MetricDataType: metricValues[0].DataType(),
			MetricUnit:     metricValues[0].Unit(),
		},
		{
			MetricName:     metricName2,
			MetricDataType: metricValues[0].DataType(),
			MetricUnit:     metricValues[0].Unit(),
		},
	}

	expectedGroups := map[MetricsDataPointKey][]*MetricsDataPoint{
		expectedGroupingKeys[0]: {
			dataPoints[0], dataPoints[1], dataPoints[4], dataPoints[5],
		},
		expectedGroupingKeys[1]: {
			dataPoints[2], dataPoints[3], dataPoints[6], dataPoints[7],
		},
	}

	return testData{dataPoints: dataPoints, expectedGroupingKeys: expectedGroupingKeys, expectedGroups: expectedGroups}
}

func newMetricDataPoint(metricName string, timestamp time.Time, labelValues []LabelValue, metricValue MetricValue) *MetricsDataPoint {
	return &MetricsDataPoint{
		metricName:  metricName,
		timestamp:   timestamp,
		databaseID:  databaseID(),
		labelValues: labelValues,
		metricValue: metricValue,
	}
}
