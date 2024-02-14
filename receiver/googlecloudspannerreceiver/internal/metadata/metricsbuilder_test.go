// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/filter"
)

const (
	metricName1 = "metricName1"
	metricName2 = "metricName2"
)

type mockItemFilterResolver struct {
	mock.Mock
}

func (r *mockItemFilterResolver) Resolve(string) (filter.ItemFilter, error) {
	args := r.Called()
	return args.Get(0).(filter.ItemFilter), args.Error(1)

}

func (r *mockItemFilterResolver) Shutdown() error {
	args := r.Called()
	return args.Error(0)
}

type errorFilter struct {
}

func (f errorFilter) Filter(_ []*filter.Item) ([]*filter.Item, error) {
	return nil, errors.New("error on filter")
}

func (f errorFilter) Shutdown() error {
	return nil
}

func (f errorFilter) TotalLimit() int {
	return 0
}

func (f errorFilter) LimitByTimestamp() int {
	return 0
}

type testData struct {
	dataPoints           []*MetricsDataPoint
	expectedGroupingKeys []MetricsDataPointKey
	expectedGroups       map[MetricsDataPointKey][]*MetricsDataPoint
}

func TestNewMetricsFromDataPointBuilder(t *testing.T) {
	filterResolver := filter.NewNopItemFilterResolver()

	builder := NewMetricsFromDataPointBuilder(filterResolver)
	builderCasted := builder.(*metricsFromDataPointBuilder)
	defer executeShutdown(t, builderCasted, false)

	assert.Equal(t, filterResolver, builderCasted.filterResolver)
}

func TestMetricsFromDataPointBuilder_Build(t *testing.T) {
	testCases := map[string]struct {
		metricsDataType pmetric.MetricType
		expectedError   error
	}{
		"Gauge":                      {pmetric.MetricTypeGauge, nil},
		"Sum":                        {pmetric.MetricTypeSum, nil},
		"Gauge with filtering error": {pmetric.MetricTypeGauge, errors.New("filtering error")},
		"Sum with filtering error":   {pmetric.MetricTypeSum, errors.New("filtering error")},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			testMetricsFromDataPointBuilderBuild(t, testCase.metricsDataType, testCase.expectedError)
		})
	}
}

func testMetricsFromDataPointBuilderBuild(t *testing.T, metricDataType pmetric.MetricType, expectedError error) {
	filterResolver := &mockItemFilterResolver{}
	dataForTesting := generateTestData(metricDataType)
	builder := &metricsFromDataPointBuilder{filterResolver: filterResolver}
	defer executeMockedShutdown(t, builder, filterResolver, expectedError)
	expectedGroupingKeysByMetricName := make(map[string]MetricsDataPointKey, len(dataForTesting.expectedGroupingKeys))

	for _, expectedGroupingKey := range dataForTesting.expectedGroupingKeys {
		expectedGroupingKeysByMetricName[expectedGroupingKey.MetricName] = expectedGroupingKey
	}

	if expectedError != nil {
		filterResolver.On("Resolve").Return(errorFilter{}, nil)
	} else {
		filterResolver.On("Resolve").Return(filter.NewNopItemCardinalityFilter(), nil)
	}

	metric, err := builder.Build(dataForTesting.dataPoints)

	filterResolver.AssertExpectations(t)

	if expectedError != nil {
		require.Error(t, err)
		return
	}
	require.NoError(t, err)

	assert.Equal(t, len(dataForTesting.dataPoints), metric.DataPointCount())
	assert.Equal(t, len(dataForTesting.expectedGroups), metric.MetricCount())
	assert.Equal(t, 1, metric.ResourceMetrics().At(0).ScopeMetrics().Len())
	assert.Equal(t, len(dataForTesting.expectedGroups), metric.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len())
	require.Equal(t, instrumentationLibraryName, metric.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Name())

	for i := 0; i < len(dataForTesting.expectedGroups); i++ {
		ilMetric := metric.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(i)
		expectedGroupingKey := expectedGroupingKeysByMetricName[ilMetric.Name()]
		expectedDataPoints := dataForTesting.expectedGroups[expectedGroupingKey]

		for dataPointIndex, expectedDataPoint := range expectedDataPoints {
			assert.Equal(t, expectedDataPoint.metricName, ilMetric.Name())
			assert.Equal(t, expectedDataPoint.metricValue.Metadata().Unit(), ilMetric.Unit())
			assert.Equal(t, expectedDataPoint.metricValue.Metadata().DataType().MetricType(), ilMetric.Type())

			var dataPoint pmetric.NumberDataPoint

			if metricDataType == pmetric.MetricTypeGauge {
				assert.NotNil(t, ilMetric.Gauge())
				assert.Equal(t, len(expectedDataPoints), ilMetric.Gauge().DataPoints().Len())
				dataPoint = ilMetric.Gauge().DataPoints().At(dataPointIndex)
			} else {
				assert.NotNil(t, ilMetric.Sum())
				assert.Equal(t, pmetric.AggregationTemporalityDelta, ilMetric.Sum().AggregationTemporality())
				assert.True(t, ilMetric.Sum().IsMonotonic())
				assert.Equal(t, len(expectedDataPoints), ilMetric.Sum().DataPoints().Len())
				dataPoint = ilMetric.Sum().DataPoints().At(dataPointIndex)
			}

			assertMetricValue(t, expectedDataPoint.metricValue, dataPoint)

			assert.Equal(t, pcommon.NewTimestampFromTime(expectedDataPoint.timestamp), dataPoint.Timestamp())
			// Adding +3 here because we'll always have 3 labels added for each metric: project_id, instance_id, database
			assert.Equal(t, 3+len(expectedDataPoint.labelValues), dataPoint.Attributes().Len())

			attributesMap := dataPoint.Attributes()

			assertDefaultLabels(t, attributesMap, expectedDataPoint.databaseID)
			assertNonDefaultLabels(t, attributesMap, expectedDataPoint.labelValues)
		}
	}
}

func TestMetricsFromDataPointBuilder_GroupAndFilter(t *testing.T) {
	testCases := map[string]struct {
		expectedError error
	}{
		"Happy path":           {nil},
		"With filtering error": {errors.New("filtering error")},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			filterResolver := &mockItemFilterResolver{}
			builder := &metricsFromDataPointBuilder{
				filterResolver: filterResolver,
			}
			defer executeMockedShutdown(t, builder, filterResolver, testCase.expectedError)
			dataForTesting := generateTestData(metricDataType)

			if testCase.expectedError != nil {
				filterResolver.On("Resolve").Return(errorFilter{}, nil)
			} else {
				filterResolver.On("Resolve").Return(filter.NewNopItemCardinalityFilter(), testCase.expectedError)
			}

			groupedDataPoints, err := builder.groupAndFilter(dataForTesting.dataPoints)

			filterResolver.AssertExpectations(t)

			if testCase.expectedError != nil {
				require.Error(t, err)
				require.Nil(t, groupedDataPoints)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, groupedDataPoints)

			assert.Equal(t, len(dataForTesting.expectedGroups), len(groupedDataPoints))

			for expectedGroupingKey, expectedGroupPoints := range dataForTesting.expectedGroups {
				dataPointsByKey := groupedDataPoints[expectedGroupingKey]

				assert.Equal(t, len(expectedGroupPoints), len(dataPointsByKey))

				for i, point := range expectedGroupPoints {
					assert.Equal(t, point, dataPointsByKey[i])
				}
			}
		})
	}
}

func TestMetricsFromDataPointBuilder_GroupAndFilter_NilDataPoints(t *testing.T) {
	builder := &metricsFromDataPointBuilder{
		filterResolver: filter.NewNopItemFilterResolver(),
	}
	defer executeShutdown(t, builder, false)

	groupedDataPoints, err := builder.groupAndFilter(nil)

	require.NoError(t, err)

	assert.Equal(t, 0, len(groupedDataPoints))
}

func TestMetricsFromDataPointBuilder_Filter(t *testing.T) {
	dataForTesting := generateTestData(metricDataType)
	testCases := map[string]struct {
		expectedError error
	}{
		"Happy path":       {nil},
		"Error on resolve": {errors.New("error on resolve")},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			filterResolver := &mockItemFilterResolver{}
			builder := &metricsFromDataPointBuilder{
				filterResolver: filterResolver,
			}
			defer executeMockedShutdown(t, builder, filterResolver, testCase.expectedError)

			if testCase.expectedError != nil {
				filterResolver.On("Resolve").Return(errorFilter{}, testCase.expectedError)
			} else {
				filterResolver.On("Resolve").Return(filter.NewNopItemCardinalityFilter(), testCase.expectedError)
			}

			filteredDataPoints, err := builder.filter(metricName1, dataForTesting.dataPoints)

			filterResolver.AssertExpectations(t)

			if testCase.expectedError != nil {
				require.Error(t, err)
				require.Nil(t, filteredDataPoints)
			} else {
				require.NoError(t, err)
				assert.Equal(t, dataForTesting.dataPoints, filteredDataPoints)
			}
		})
	}
}

func TestMetricsFromDataPointBuilder_Shutdown(t *testing.T) {
	testCases := map[string]struct {
		expectedError error
	}{
		"Happy path": {nil},
		"Error":      {errors.New("shutdown error")},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			filterResolver := &mockItemFilterResolver{}
			builder := &metricsFromDataPointBuilder{
				filterResolver: filterResolver,
			}

			executeMockedShutdown(t, builder, filterResolver, testCase.expectedError)
		})
	}
}

func generateTestData(metricDataType pmetric.MetricType) testData {
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
			MetricName: metricName1,
			MetricType: metricValues[0].Metadata().DataType(),
			MetricUnit: metricValues[0].Metadata().Unit(),
		},
		{
			MetricName: metricName2,
			MetricType: metricValues[0].Metadata().DataType(),
			MetricUnit: metricValues[0].Metadata().Unit(),
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

func executeShutdown(t *testing.T, metricsBuilder MetricsBuilder, expectError bool) {
	err := metricsBuilder.Shutdown()
	if expectError {
		require.Error(t, err)
	} else {
		require.NoError(t, err)
	}
}

func executeMockedShutdown(t *testing.T, metricsBuilder MetricsBuilder, filterResolver *mockItemFilterResolver,
	expectedError error) {

	filterResolver.On("Shutdown").Return(expectedError)
	_ = metricsBuilder.Shutdown()
	filterResolver.AssertExpectations(t)
}
