// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterfactory

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver/internal/filter"
)

func TestFilterBuilder_BuildFilterByMetricZeroTotalLimit(t *testing.T) {
	logger := zaptest.NewLogger(t)
	metricPrefixes := []string{prefix1, prefix2}
	prefixHighCardinality := []bool{true, true}
	metadataItems := generateMetadataItems(metricPrefixes, prefixHighCardinality)
	config := &ItemFilterFactoryConfig{
		MetadataItems: metadataItems,
	}
	nopItemFilter := filter.NewNopItemCardinalityFilter()
	builder := filterBuilder{
		logger: logger,
		config: config,
	}

	result := builder.buildFilterByMetricZeroTotalLimit()

	// Because we have 2 groups and each group has 2 metrics
	assert.Equal(t, len(metricPrefixes)*2, len(result))
	for _, metadataItem := range metadataItems {
		for _, metricValueMetadata := range metadataItem.QueryMetricValuesMetadata {
			f, exists := result[metadataItem.MetricNamePrefix+metricValueMetadata.Name()]
			assert.True(t, exists)
			assert.Equal(t, nopItemFilter, f)
		}
	}
}

func TestFilterBuilder_BuildFilterByMetricPositiveTotalLimit(t *testing.T) {
	logger := zaptest.NewLogger(t)
	testCases := map[string]struct {
		metricPrefixes                          []string
		prefixHighCardinality                   []bool
		totalLimit                              int
		projectAmount                           int
		instanceAmount                          int
		databaseAmount                          int
		expectedHighCardinalityTotalLimit       int
		expectedHighCardinalityLimitByTimestamp int
		expectError                             bool
	}{
		"Happy path with 2 high cardinality groups":                                 {[]string{prefix1, prefix2}, []bool{true, true}, 200 * defaultMetricDataPointsAmountInPeriod, 1, 2, 5, 72000, 50, false},
		"Happy path with 2 low cardinality groups":                                  {[]string{prefix1, prefix2}, []bool{false, false}, 200, 1, 2, 5, 0, 0, false},
		"Happy path with 1 low and 1 high cardinality groups":                       {[]string{prefix1, prefix2}, []bool{false, true}, 200*defaultMetricDataPointsAmountInPeriod + 20, 1, 2, 5, 144000, 100, false},
		"Error when limit by timestamp is lower than 1 for high cardinality groups": {[]string{prefix1, prefix2}, []bool{true, true}, 200, 1, 2, 5, 0, 0, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			metadataItems := generateMetadataItems(testCase.metricPrefixes, testCase.prefixHighCardinality)
			config := &ItemFilterFactoryConfig{
				MetadataItems:  metadataItems,
				TotalLimit:     testCase.totalLimit,
				ProjectAmount:  testCase.projectAmount,
				InstanceAmount: testCase.instanceAmount,
				DatabaseAmount: testCase.databaseAmount,
			}
			builder := filterBuilder{
				logger: logger,
				config: config,
			}

			result, err := builder.buildFilterByMetricPositiveTotalLimit()
			if testCase.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Because we have 2 groups and each group has 2 metrics
			assert.Equal(t, len(testCase.metricPrefixes)*2, len(result))
			for _, metadataItem := range metadataItems {
				for _, metricValueMetadata := range metadataItem.QueryMetricValuesMetadata {
					f, exists := result[metadataItem.MetricNamePrefix+metricValueMetadata.Name()]
					assert.True(t, exists)
					if metadataItem.HighCardinality {
						assert.Equal(t, testCase.expectedHighCardinalityTotalLimit, f.TotalLimit())
						assert.Equal(t, testCase.expectedHighCardinalityLimitByTimestamp, f.LimitByTimestamp())
					} else {
						// For low cardinality group both limits are equal to projectAmount * instanceAmount * databaseAmount
						expectedLimit := testCase.projectAmount * testCase.instanceAmount * testCase.databaseAmount
						assert.Equal(t, expectedLimit, f.TotalLimit())
						assert.Equal(t, expectedLimit, f.LimitByTimestamp())
					}
				}
			}
		})
	}
}

func TestFilterBuilder_HandleLowCardinalityGroups(t *testing.T) {
	logger := zaptest.NewLogger(t)
	testCases := map[string]struct {
		metricPrefixes              []string
		prefixHighCardinality       []bool
		totalLimit                  int
		projectAmount               int
		instanceAmount              int
		databaseAmount              int
		expectedRemainingTotalLimit int
	}{
		"With 2 low cardinality groups": {[]string{prefix1, prefix2}, []bool{false, false}, 50, 1, 2, 5, 10},
		"With 0 low cardinality groups": {[]string{}, []bool{}, 50, 1, 2, 5, 50},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			metadataItems := generateMetadataItems(testCase.metricPrefixes, testCase.prefixHighCardinality)
			config := &ItemFilterFactoryConfig{
				MetadataItems:  metadataItems,
				TotalLimit:     testCase.totalLimit,
				ProjectAmount:  testCase.projectAmount,
				InstanceAmount: testCase.instanceAmount,
				DatabaseAmount: testCase.databaseAmount,
			}
			builder := filterBuilder{
				logger: logger,
				config: config,
			}

			filterByMetric := make(map[string]filter.ItemFilter)
			remainingTotalLimit, err := builder.handleLowCardinalityGroups(metadataItems, testCase.totalLimit, filterByMetric)
			require.NoError(t, err)

			// Because we have 2 groups and each group has 2 metrics
			assert.Equal(t, len(testCase.metricPrefixes)*2, len(filterByMetric))
			for _, metadataItem := range metadataItems {
				for _, metricValueMetadata := range metadataItem.QueryMetricValuesMetadata {
					f, exists := filterByMetric[metadataItem.MetricNamePrefix+metricValueMetadata.Name()]
					assert.True(t, exists)
					// For low cardinality group both limits are equal to projectAmount * instanceAmount * databaseAmount
					expectedLimit := testCase.projectAmount * testCase.instanceAmount * testCase.databaseAmount
					assert.Equal(t, expectedLimit, f.TotalLimit())
					assert.Equal(t, expectedLimit, f.LimitByTimestamp())
					assert.Equal(t, testCase.expectedRemainingTotalLimit, remainingTotalLimit)
				}
			}
		})
	}
}

func TestFilterBuilder_HandleHighCardinalityGroups(t *testing.T) {
	logger := zaptest.NewLogger(t)
	testCases := map[string]struct {
		metricPrefixes                          []string
		prefixHighCardinality                   []bool
		totalLimit                              int
		expectedHighCardinalityTotalLimit       int
		expectedHighCardinalityLimitByTimestamp int
		expectedRemainingTotalLimit             int
		expectError                             bool
	}{
		"With 2 high cardinality groups":                                            {[]string{prefix1, prefix2}, []bool{true, true}, 200 * defaultMetricDataPointsAmountInPeriod, 72000, 50, 0, false},
		"With zero high cardinality groups":                                         {[]string{}, []bool{}, 200, 0, 0, 200, false},
		"Error when limit by timestamp is lower than 1 for high cardinality groups": {[]string{prefix1, prefix2}, []bool{true, true}, 200, 0, 0, 200, true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			metadataItems := generateMetadataItems(testCase.metricPrefixes, testCase.prefixHighCardinality)
			config := &ItemFilterFactoryConfig{
				MetadataItems:  metadataItems,
				TotalLimit:     testCase.totalLimit,
				ProjectAmount:  1,
				InstanceAmount: 2,
				DatabaseAmount: 5,
			}
			builder := filterBuilder{
				logger: logger,
				config: config,
			}
			filterByMetric := make(map[string]filter.ItemFilter)
			remainingTotalLimit, err := builder.handleHighCardinalityGroups(metadataItems, testCase.totalLimit, filterByMetric)
			if testCase.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Because we have 2 groups and each group has 2 metrics
			assert.Equal(t, len(testCase.metricPrefixes)*2, len(filterByMetric))
			for _, metadataItem := range metadataItems {
				for _, metricValueMetadata := range metadataItem.QueryMetricValuesMetadata {
					f, exists := filterByMetric[metadataItem.MetricNamePrefix+metricValueMetadata.Name()]
					assert.True(t, exists)
					assert.Equal(t, testCase.expectedHighCardinalityTotalLimit, f.TotalLimit())
					assert.Equal(t, testCase.expectedHighCardinalityLimitByTimestamp, f.LimitByTimestamp())
					assert.Equal(t, testCase.expectedRemainingTotalLimit, remainingTotalLimit)
				}
			}
		})
	}
}

func TestFilterBuilder_TestConstructFiltersForGroups(t *testing.T) {
	logger := zaptest.NewLogger(t)
	metricPrefixes := []string{prefix1, prefix2}
	prefixHighCardinality := []bool{true, true}
	metadataItems := generateMetadataItems(metricPrefixes, prefixHighCardinality)
	config := &ItemFilterFactoryConfig{
		MetadataItems: metadataItems,
	}
	builder := filterBuilder{
		logger: logger,
		config: config,
	}
	filterByMetric := make(map[string]filter.ItemFilter)
	const totalLimitPerMetric, limitPerMetricByTimestamp, remainingTotalLimit, expectedRemainingTotalLimit = 50, 10, 200, 0

	result, err := builder.constructFiltersForGroups(totalLimitPerMetric, limitPerMetricByTimestamp, metadataItems,
		remainingTotalLimit, filterByMetric)
	require.NoError(t, err)

	// Because we have 2 groups and each group has 2 metrics
	assert.Equal(t, len(metricPrefixes)*2, len(filterByMetric))
	for _, metadataItem := range metadataItems {
		for _, metricValueMetadata := range metadataItem.QueryMetricValuesMetadata {
			f, exists := filterByMetric[metadataItem.MetricNamePrefix+metricValueMetadata.Name()]
			assert.True(t, exists)
			assert.Equal(t, totalLimitPerMetric, f.TotalLimit())
			assert.Equal(t, limitPerMetricByTimestamp, f.LimitByTimestamp())
			assert.Equal(t, expectedRemainingTotalLimit, result)
		}
	}
}

func TestCountMetricsInGroups(t *testing.T) {
	metricPrefixes := []string{prefix1, prefix2}
	prefixHighCardinality := []bool{true, true}
	metadataItems := generateMetadataItems(metricPrefixes, prefixHighCardinality)

	assert.Equal(t, 4, countMetricsInGroups(metadataItems))
}

func TestGroupByCardinality(t *testing.T) {
	metricPrefixes := []string{"prefix1-", "prefix2-"}
	prefixHighCardinality := []bool{false, true}
	metadataItems := generateMetadataItems(metricPrefixes, prefixHighCardinality)

	result := groupByCardinality(metadataItems)

	assert.Equal(t, 2, len(result))

	for _, metadataItem := range metadataItems {
		groups, exists := result[metadataItem.HighCardinality]
		assert.True(t, exists)
		assert.Equal(t, 1, len(groups))
		assert.Equal(t, metadataItem, groups[0])
	}
}
